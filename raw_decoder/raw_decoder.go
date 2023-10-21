package raw_decoder

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	db2 "golbat/db"
	"golbat/grpc"
	"golbat/pogo"
	"golbat/stats_collector"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type RawDecoder interface {
	GetProtoDataFromHTTP(http.Header, []byte) (*ProtoData, error)
	GetProtoDataFromGRPC(*grpc.RawProtoRequest) *ProtoData
	Decode(context.Context, *ProtoData)
}

type ProtoData struct {
	*CommonData
	Protos []Proto
}

func (pd ProtoData) Lat() float64 {
	l := len(pd.Protos)
	if l == 0 {
		return 0
	}
	return pd.Protos[l-1].Lat
}

func (pd ProtoData) Lon() float64 {
	l := len(pd.Protos)
	if l == 0 {
		return 0
	}
	return pd.Protos[l-1].Lon
}

type CommonData struct {
	Account     string
	Level       int
	Uuid        string
	ScanContext string
}

type Proto struct {
	*CommonData
	Method         int
	HaveAr         *bool
	Lat            float64
	Lon            float64
	base64Request  string
	base64Response string
	requestBytes   []byte
	responseBytes  []byte
}

func (pd *Proto) RequestProtoBytes() []byte {
	if pd.requestBytes != nil {
		return pd.requestBytes
	}
	if pd.base64Request == "" {
		return nil
	}
	reqBytes, err := base64.StdEncoding.DecodeString(pd.base64Request)
	if err != nil {
		return nil
	}
	pd.requestBytes = reqBytes
	return reqBytes
}

func (pd *Proto) ResponseProtoBytes() []byte {
	if pd.responseBytes != nil {
		return pd.responseBytes
	}
	if pd.base64Response == "" {
		return nil
	}
	respBytes, err := base64.StdEncoding.DecodeString(pd.base64Response)
	if err != nil {
		return nil
	}
	pd.responseBytes = respBytes
	return respBytes
}

type decodeProtoMethod struct {
	Decode   func(*rawDecoder, context.Context, *Proto) (bool, string)
	MinLevel int
}

var decodeMethods = map[pogo.Method]*decodeProtoMethod{
	pogo.Method_METHOD_START_INCIDENT:                                &decodeProtoMethod{(*rawDecoder).decodeStartIncident, 30},
	pogo.Method_METHOD_INVASION_OPEN_COMBAT_SESSION:                  &decodeProtoMethod{(*rawDecoder).decodeOpenInvasion, 30},
	pogo.Method_METHOD_FORT_DETAILS:                                  &decodeProtoMethod{(*rawDecoder).decodeFortDetails, 30},
	pogo.Method_METHOD_GET_MAP_OBJECTS:                               &decodeProtoMethod{(*rawDecoder).decodeGMO, 0},
	pogo.Method_METHOD_GYM_GET_INFO:                                  &decodeProtoMethod{(*rawDecoder).decodeGetGymInfo, 10},
	pogo.Method_METHOD_ENCOUNTER:                                     &decodeProtoMethod{(*rawDecoder).decodeEncounter, 30},
	pogo.Method_METHOD_DISK_ENCOUNTER:                                &decodeProtoMethod{(*rawDecoder).decodeDiskEncounter, 30},
	pogo.Method_METHOD_FORT_SEARCH:                                   &decodeProtoMethod{(*rawDecoder).decodeFortSearch, 10},
	pogo.Method(pogo.ClientAction_CLIENT_ACTION_PROXY_SOCIAL_ACTION): &decodeProtoMethod{(*rawDecoder).decodeSocialAction, 0},
	pogo.Method_METHOD_GET_MAP_FORTS:                                 &decodeProtoMethod{(*rawDecoder).decodeGetMapForts, 10},
	pogo.Method_METHOD_GET_ROUTES:                                    &decodeProtoMethod{(*rawDecoder).decodeGetRoutes, 30},
	pogo.Method_METHOD_GET_CONTEST_DATA:                              &decodeProtoMethod{(*rawDecoder).decodeGetContestData, 10},
	pogo.Method_METHOD_GET_POKEMON_SIZE_CONTEST_ENTRY:                &decodeProtoMethod{(*rawDecoder).decodeGetPokemonSizeContestEntry, 10},
	// ignores
	pogo.Method_METHOD_GET_PLAYER:              nil,
	pogo.Method_METHOD_GET_HOLOHOLO_INVENTORY:  nil,
	pogo.Method_METHOD_CREATE_COMBAT_CHALLENGE: nil,
}

var _ RawDecoder = (*rawDecoder)(nil)

type rawDecoder struct {
	decodeTimeout  time.Duration
	dbDetails      db2.DbDetails
	statsCollector stats_collector.StatsCollector
}

func (dec *rawDecoder) decode(ctx context.Context, protoData *Proto) {
	getMethodName := func(method pogo.Method, trimString bool) string {
		if val, ok := pogo.Method_name[int32(method)]; ok {
			if trimString && strings.HasPrefix(val, "METHOD_") {
				return strings.TrimPrefix(val, "METHOD_")
			}
			return val
		}
		return fmt.Sprintf("#%d", method)
	}

	processed := false
	ignore := false
	start := time.Now()
	result := ""

	method := pogo.Method(protoData.Method)
	decodeMethod, ok := decodeMethods[method]
	if ok {
		if decodeMethod == nil {
			// completely ignore
			return
		}
		if protoData.Level < decodeMethod.MinLevel {
			dec.statsCollector.IncDecodeMethods("error", "low_level", getMethodName(method, true))
			log.Debugf("Insufficient Level %d Did not process hook type %s", protoData.Level, method)
			return
		}
		processed, result = decodeMethod.Decode(dec, ctx, protoData)
	} else {
		log.Debugf("Did not know hook type %s", method)
	}

	if !ignore {
		elapsed := time.Since(start)
		if processed == true {
			dec.statsCollector.IncDecodeMethods("ok", "", getMethodName(method, true))
			log.Debugf("%s/%s %s - %s - %s", protoData.Uuid, protoData.Account, pogo.Method(method), elapsed, result)
		} else {
			log.Debugf("%s/%s %s - %s - %s", protoData.Uuid, protoData.Account, pogo.Method(method), elapsed, "**Did not process**")
			dec.statsCollector.IncDecodeMethods("unprocessed", "", getMethodName(method, true))
		}
	}
}

func (dec *rawDecoder) parsePogodroidBody(headers http.Header, body []byte, origin string) (*ProtoData, error) {
	const arQuestId = int(pogo.QuestType_QUEST_GEOTARGETED_AR_SCAN)

	type pogoDroidRawEntry struct {
		Lat        float64 `json:"lat"`
		Lng        float64 `json:"lng"`
		Payload    string  `json:"payload"`
		Type       int     `json:"type"`
		QuestsHeld []int   `json:"quests_held"`
	}

	var entries []pogoDroidRawEntry

	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}

	commonData := &CommonData{
		Uuid:    origin,
		Account: "Pogodroid",
		Level:   30,
	}

	protos := make([]Proto, len(entries))

	for entryIdx, entry := range entries {
		var lat, lon float64

		if entry.Lat != 0 && entry.Lng != 0 {
			lat = entry.Lat
			lon = entry.Lng
		}

		var haveAr *bool

		if entry.QuestsHeld != nil {
			for _, quest_id := range entry.QuestsHeld {
				if quest_id == arQuestId {
					value := true
					haveAr = &value
					break
				}
			}
			if haveAr == nil {
				value := false
				haveAr = &value
			}
		}

		protos[entryIdx] = Proto{
			CommonData:     commonData,
			base64Response: entry.Payload,
			Method:         entry.Type,
			HaveAr:         haveAr,
			Lat:            lat,
			Lon:            lon,
		}
	}

	return &ProtoData{
		CommonData: commonData,
		Protos:     protos,
	}, nil
}

func decodeAlternate(data map[string]interface{}, key1, key2 string) interface{} {
	if v := data[key1]; v != nil {
		return v
	}
	if v := data[key2]; v != nil {
		return v
	}
	return nil
}

func (dec *rawDecoder) GetProtoDataFromHTTP(headers http.Header, body []byte) (*ProtoData, error) {
	if origin := headers.Get("origin"); origin != "" {
		return dec.parsePogodroidBody(headers, body, origin)
	}

	var raw map[string]any

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	commonData := &CommonData{
		Level: 30,
	}

	baseProto := Proto{
		CommonData: commonData,
	}

	if v := raw["have_ar"]; v != nil {
		res, ok := v.(bool)
		if ok {
			baseProto.HaveAr = &res
		}
	}
	if v := raw["uuid"]; v != nil {
		baseProto.Uuid, _ = v.(string)
	}
	if v := raw["username"]; v != nil {
		baseProto.Account, _ = v.(string)
	}
	if v := raw["trainerlvl"]; v != nil {
		lvl, ok := v.(float64)
		if ok {
			baseProto.Level = int(lvl)
		}
	}
	if v := raw["scan_context"]; v != nil {
		baseProto.ScanContext, _ = v.(string)
	}
	if v := raw["lat_target"]; v != nil {
		baseProto.Lat, _ = v.(float64)
	}
	if v := raw["lon_target"]; v != nil {
		baseProto.Lon, _ = v.(float64)
	}

	contents, ok := raw["contents"].([]any)
	if !ok {
		return nil, errors.New("failed to decode 'contents'")
	}

	var protos []Proto

	for _, v := range contents {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		// Try to decode the payload automatically without requiring any knowledge of the
		// provider type

		base64data := decodeAlternate(entry, "data", "payload")
		method := decodeAlternate(entry, "method", "type")
		if method == nil || base64data == nil {
			log.Errorf("Error decoding raw (no method or base64data)")
			continue
		}

		proto := baseProto
		proto.base64Response, _ = base64data.(string)
		proto.Method = func() int {
			if res, ok := method.(float64); ok {
				return int(res)
			}
			return 0
		}()

		if request := entry["request"]; request != nil {
			proto.base64Request, _ = request.(string)
		}
		if haveAr := entry["have_ar"]; haveAr != nil {
			res, ok := haveAr.(bool)
			if ok {
				proto.HaveAr = &res
			}
		}

		protos = append(protos, proto)
	}
	return &ProtoData{
		CommonData: commonData,
		Protos:     protos,
	}, nil
}

func (dec *rawDecoder) GetProtoDataFromGRPC(in *grpc.RawProtoRequest) *ProtoData {
	commonData := &CommonData{
		Uuid:    in.DeviceId,
		Account: in.Username,
		Level:   int(in.TrainerLevel),
	}

	baseProto := Proto{
		CommonData: commonData,
		Lat:        float64(in.LatTarget),
		Lon:        float64(in.LonTarget),
		HaveAr:     in.HaveAr,
	}

	if in.ScanContext != nil {
		baseProto.ScanContext = *in.ScanContext
	}

	protos := make([]Proto, len(in.Contents))
	for i, v := range in.Contents {
		proto := baseProto
		proto.Method = int(v.Method)
		proto.requestBytes = v.RequestPayload
		proto.responseBytes = v.ResponsePayload
		if v.HaveAr != nil {
			proto.HaveAr = v.HaveAr
		}
		protos[i] = proto
	}
	return &ProtoData{
		CommonData: commonData,
		Protos:     protos,
	}
}

func (dec *rawDecoder) Decode(ctx context.Context, protoData *ProtoData) {
	for _, proto := range protoData.Protos {
		// provide independent cancellation contexts for each proto decode
		ctx, cancel := context.WithTimeout(ctx, dec.decodeTimeout)
		dec.decode(ctx, &proto)
		cancel()
	}
}

func NewRawDecoder(decodeTimeout time.Duration, dbDetails db2.DbDetails, statsCollector stats_collector.StatsCollector) RawDecoder {
	return &rawDecoder{
		decodeTimeout:  decodeTimeout,
		dbDetails:      dbDetails,
		statsCollector: statsCollector,
	}
}
