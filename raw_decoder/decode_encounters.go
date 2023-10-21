package raw_decoder

import (
	"context"
	"fmt"
	"golbat/decoder"
	"golbat/pogo"

	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

func getScanParameters(protoData *Proto) decoder.ScanParameters {
	return decoder.FindScanConfiguration(protoData.ScanContext, protoData.Lat, protoData.Lon)
}

func (dec *rawDecoder) decodeEncounter(ctx context.Context, protoData *Proto) (bool, string) {
	if !getScanParameters(protoData).ProcessPokemon {
		return true, ""
	}
	response := protoData.ResponseProtoBytes()
	username := protoData.Account

	decodedEncounterInfo := &pogo.EncounterOutProto{}
	if err := proto.Unmarshal(response, decodedEncounterInfo); err != nil {
		log.Errorf("Failed to parse %s", err)
		dec.statsCollector.IncDecodeEncounter("error", "parse")
		return true, fmt.Sprintf("Failed to parse %s", err)
	}

	if decodedEncounterInfo.Status != pogo.EncounterOutProto_ENCOUNTER_SUCCESS {
		dec.statsCollector.IncDecodeEncounter("error", "non_success")
		res := fmt.Sprintf(`GymGetInfoOutProto: Ignored non-success value %d:%s`, decodedEncounterInfo.Status,
			pogo.EncounterOutProto_Status_name[int32(decodedEncounterInfo.Status)])
		return true, res
	}

	dec.statsCollector.IncDecodeEncounter("ok", "")
	return true, decoder.UpdatePokemonRecordWithEncounterProto(ctx, dec.dbDetails, decodedEncounterInfo, username)
}

func (dec *rawDecoder) decodeDiskEncounter(ctx context.Context, protoData *Proto) (bool, string) {
	response := protoData.ResponseProtoBytes()
	decodedEncounterInfo := &pogo.DiskEncounterOutProto{}
	if err := proto.Unmarshal(response, decodedEncounterInfo); err != nil {
		log.Errorf("Failed to parse %s", err)
		dec.statsCollector.IncDecodeDiskEncounter("error", "parse")
		return true, fmt.Sprintf("Failed to parse %s", err)
	}

	if decodedEncounterInfo.Result != pogo.DiskEncounterOutProto_SUCCESS {
		dec.statsCollector.IncDecodeDiskEncounter("error", "non_success")
		res := fmt.Sprintf(`DiskEncounterOutProto: Ignored non-success value %d:%s`, decodedEncounterInfo.Result,
			pogo.DiskEncounterOutProto_Result_name[int32(decodedEncounterInfo.Result)])
		return true, res
	}

	dec.statsCollector.IncDecodeDiskEncounter("ok", "")
	return true, decoder.UpdatePokemonRecordWithDiskEncounterProto(ctx, dec.dbDetails, decodedEncounterInfo)
}
