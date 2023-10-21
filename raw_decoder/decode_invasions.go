package raw_decoder

import (
	"context"
	"fmt"
	"golbat/decoder"
	"golbat/pogo"

	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

func (dec *rawDecoder) decodeStartIncident(ctx context.Context, protoData *Proto) (bool, string) {
	sDec := protoData.ResponseProtoBytes()
	decodedIncident := &pogo.StartIncidentOutProto{}
	if err := proto.Unmarshal(sDec, decodedIncident); err != nil {
		log.Errorf("Failed to parse %s", err)
		dec.statsCollector.IncDecodeStartIncident("error", "parse")
		return true, fmt.Sprintf("Failed to parse %s", err)
	}

	if decodedIncident.Status != pogo.StartIncidentOutProto_SUCCESS {
		dec.statsCollector.IncDecodeStartIncident("error", "non_success")
		res := fmt.Sprintf(`GiovanniOutProto: Ignored non-success value %d:%s`, decodedIncident.Status,
			pogo.StartIncidentOutProto_Status_name[int32(decodedIncident.Status)])
		return true, res
	}

	dec.statsCollector.IncDecodeStartIncident("ok", "")
	return true, decoder.ConfirmIncident(ctx, dec.dbDetails, decodedIncident)
}

func (dec *rawDecoder) decodeOpenInvasion(ctx context.Context, protoData *Proto) (bool, string) {
	request := protoData.RequestProtoBytes()
	if request == nil {
		return false, ""
	}
	response := protoData.ResponseProtoBytes()

	decodeOpenInvasionRequest := &pogo.OpenInvasionCombatSessionProto{}

	if err := proto.Unmarshal(request, decodeOpenInvasionRequest); err != nil {
		log.Errorf("Failed to parse %s", err)
		dec.statsCollector.IncDecodeOpenInvasion("error", "parse")
		return true, fmt.Sprintf("Failed to parse %s", err)
	}
	if decodeOpenInvasionRequest.IncidentLookup == nil {
		return true, "Invalid OpenInvasionCombatSessionProto received"
	}

	decodedOpenInvasionResponse := &pogo.OpenInvasionCombatSessionOutProto{}
	if err := proto.Unmarshal(response, decodedOpenInvasionResponse); err != nil {
		log.Errorf("Failed to parse %s", err)
		dec.statsCollector.IncDecodeOpenInvasion("error", "parse")
		return true, fmt.Sprintf("Failed to parse %s", err)
	}

	if decodedOpenInvasionResponse.Status != pogo.InvasionStatus_SUCCESS {
		dec.statsCollector.IncDecodeOpenInvasion("error", "non_success")
		res := fmt.Sprintf(`InvasionLineupOutProto: Ignored non-success value %d:%s`, decodedOpenInvasionResponse.Status,
			pogo.InvasionStatus_Status_name[int32(decodedOpenInvasionResponse.Status)])
		return true, res
	}

	dec.statsCollector.IncDecodeOpenInvasion("ok", "")
	return true, decoder.UpdateIncidentLineup(ctx, dec.dbDetails, decodeOpenInvasionRequest, decodedOpenInvasionResponse)
}
