package raw_decoder

import (
	"context"
	"fmt"
	"golbat/decoder"
	"golbat/pogo"

	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

func (dec *rawDecoder) decodeGetContestData(ctx context.Context, protoData *Proto) (bool, string) {
	// Request helps, but can be decoded without it
	request := protoData.RequestProtoBytes()
	response := protoData.ResponseProtoBytes()
	var decodedContestData pogo.GetContestDataOutProto

	if err := proto.Unmarshal(response, &decodedContestData); err != nil {
		log.Errorf("Failed to parse GetContestDataOutProto %s", err)
		return true, fmt.Sprintf("Failed to parse GetContestDataOutProto %s", err)
	}

	var decodedContestDataRequest pogo.GetContestDataProto
	if request != nil {
		if err := proto.Unmarshal(request, &decodedContestDataRequest); err != nil {
			log.Errorf("Failed to parse GetContestDataProto %s", err)
			return true, fmt.Sprintf("Failed to parse GetContestDataProto %s", err)
		}
	}
	return true, decoder.UpdatePokestopWithContestData(ctx, dec.dbDetails, &decodedContestDataRequest, &decodedContestData)
}

func (dec *rawDecoder) decodeGetPokemonSizeContestEntry(ctx context.Context, protoData *Proto) (bool, string) {
	request := protoData.RequestProtoBytes()
	response := protoData.ResponseProtoBytes()

	var decodedPokemonSizeContestEntry pogo.GetPokemonSizeLeaderboardEntryOutProto
	if err := proto.Unmarshal(response, &decodedPokemonSizeContestEntry); err != nil {
		log.Errorf("Failed to parse GetPokemonSizeLeaderboardEntryOutProto %s", err)
		return true, fmt.Sprintf("Failed to parse GetPokemonSizeLeaderboardEntryOutProto %s", err)
	}

	if decodedPokemonSizeContestEntry.Status != pogo.GetPokemonSizeLeaderboardEntryOutProto_SUCCESS {
		return true, fmt.Sprintf("Ignored GetPokemonSizeLeaderboardEntryOutProto non-success status %s", decodedPokemonSizeContestEntry.Status)
	}

	var decodedPokemonSizeContestEntryRequest pogo.GetPokemonSizeLeaderboardEntryProto
	if request != nil {
		if err := proto.Unmarshal(request, &decodedPokemonSizeContestEntryRequest); err != nil {
			log.Errorf("Failed to parse GetPokemonSizeLeaderboardEntryOutProto %s", err)
			return true, fmt.Sprintf("Failed to parse GetPokemonSizeLeaderboardEntryOutProto %s", err)
		}
	}

	return true, decoder.UpdatePokestopWithPokemonSizeContestEntry(ctx, dec.dbDetails, &decodedPokemonSizeContestEntryRequest, &decodedPokemonSizeContestEntry)
}
