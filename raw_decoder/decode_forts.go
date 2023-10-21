package raw_decoder

import (
	"context"
	"fmt"
	"golbat/decoder"
	"golbat/pogo"

	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

func (dec *rawDecoder) decodeFortSearch(ctx context.Context, protoData *Proto) (bool, string) {
	response := protoData.ResponseProtoBytes()
	haveAr := protoData.HaveAr

	if haveAr == nil {
		dec.statsCollector.IncDecodeQuest("error", "missing_ar_info")
		log.Infoln("Cannot determine AR quest - ignoring")
		// We should either assume AR quest, or trace inventory like RDM probably
		return true, "No AR quest info"
	}
	decodedQuest := &pogo.FortSearchOutProto{}
	if err := proto.Unmarshal(response, decodedQuest); err != nil {
		log.Errorf("Failed to parse %s", err)
		dec.statsCollector.IncDecodeQuest("error", "parse")
		return true, "Parse failure"
	}

	if decodedQuest.Result != pogo.FortSearchOutProto_SUCCESS {
		dec.statsCollector.IncDecodeQuest("error", "non_success")
		res := fmt.Sprintf(`GymGetInfoOutProto: Ignored non-success value %d:%s`, decodedQuest.Result,
			pogo.FortSearchOutProto_Result_name[int32(decodedQuest.Result)])
		return true, res
	}

	return true, decoder.UpdatePokestopWithQuest(ctx, dec.dbDetails, decodedQuest, *haveAr)
}

func (dec *rawDecoder) decodeFortDetails(ctx context.Context, protoData *Proto) (bool, string) {
	response := protoData.ResponseProtoBytes()
	decodedFort := &pogo.FortDetailsOutProto{}
	if err := proto.Unmarshal(response, decodedFort); err != nil {
		log.Errorf("Failed to parse %s", err)
		dec.statsCollector.IncDecodeFortDetails("error", "parse")
		return true, fmt.Sprintf("Failed to parse %s", err)
	}

	switch decodedFort.FortType {
	case pogo.FortType_CHECKPOINT:
		dec.statsCollector.IncDecodeFortDetails("ok", "pokestop")
		return true, decoder.UpdatePokestopRecordWithFortDetailsOutProto(ctx, dec.dbDetails, decodedFort)
	case pogo.FortType_GYM:
		dec.statsCollector.IncDecodeFortDetails("ok", "gym")
		return true, decoder.UpdateGymRecordWithFortDetailsOutProto(ctx, dec.dbDetails, decodedFort)
	}

	dec.statsCollector.IncDecodeFortDetails("ok", "unknown")
	return true, "Unknown fort type"
}

func (dec *rawDecoder) decodeGetMapForts(ctx context.Context, protoData *Proto) (bool, string) {
	response := protoData.ResponseProtoBytes()
	decodedMapForts := &pogo.GetMapFortsOutProto{}
	if err := proto.Unmarshal(response, decodedMapForts); err != nil {
		log.Errorf("Failed to parse %s", err)
		dec.statsCollector.IncDecodeGetMapForts("error", "parse")
		return true, fmt.Sprintf("Failed to parse %s", err)
	}

	if decodedMapForts.Status != pogo.GetMapFortsOutProto_SUCCESS {
		dec.statsCollector.IncDecodeGetMapForts("error", "non_success")
		res := fmt.Sprintf(`GetMapFortsOutProto: Ignored non-success value %d:%s`, decodedMapForts.Status,
			pogo.GetMapFortsOutProto_Status_name[int32(decodedMapForts.Status)])
		return true, res
	}

	dec.statsCollector.IncDecodeGetMapForts("ok", "")
	var outputString string
	processedForts := 0

	for _, fort := range decodedMapForts.Fort {
		status, output := decoder.UpdateFortRecordWithGetMapFortsOutProto(ctx, dec.dbDetails, fort)
		if status {
			processedForts += 1
			outputString += output + ", "
		}
	}

	if processedForts > 0 {
		return true, fmt.Sprintf("Updated %d forts: %s", processedForts, outputString)
	}
	return true, "No forts updated"
}

func (dec *rawDecoder) decodeGetGymInfo(ctx context.Context, protoData *Proto) (bool, string) {
	response := protoData.ResponseProtoBytes()
	decodedGymInfo := &pogo.GymGetInfoOutProto{}
	if err := proto.Unmarshal(response, decodedGymInfo); err != nil {
		log.Errorf("Failed to parse %s", err)
		dec.statsCollector.IncDecodeGetGymInfo("error", "parse")
		return true, fmt.Sprintf("Failed to parse %s", err)
	}

	if decodedGymInfo.Result != pogo.GymGetInfoOutProto_SUCCESS {
		dec.statsCollector.IncDecodeGetGymInfo("error", "non_success")
		res := fmt.Sprintf(`GymGetInfoOutProto: Ignored non-success value %d:%s`, decodedGymInfo.Result,
			pogo.GymGetInfoOutProto_Result_name[int32(decodedGymInfo.Result)])
		return true, res
	}

	dec.statsCollector.IncDecodeGetGymInfo("ok", "")
	return true, decoder.UpdateGymRecordWithGymInfoProto(ctx, dec.dbDetails, decodedGymInfo)
}
