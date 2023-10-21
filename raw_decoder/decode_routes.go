package raw_decoder

import (
	"context"
	"fmt"
	"golbat/decoder"
	"golbat/pogo"

	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

func (dec *rawDecoder) decodeGetRoutes(ctx context.Context, protoData *Proto) (bool, string) {
	response := protoData.ResponseProtoBytes()
	getRoutesOutProto := &pogo.GetRoutesOutProto{}
	if err := proto.Unmarshal(response, getRoutesOutProto); err != nil {
		return true, fmt.Sprintf("failed to decode GetRoutesOutProto %s", err)
	}

	if getRoutesOutProto.Status != pogo.GetRoutesOutProto_SUCCESS {
		return true, fmt.Sprintf("GetRoutesOutProto: Ignored non-success value %d:%s", getRoutesOutProto.Status, getRoutesOutProto.Status.String())
	}

	decodeSuccesses := map[string]bool{}
	decodeErrors := map[string]bool{}

	for _, routeMapCell := range getRoutesOutProto.GetRouteMapCell() {
		for _, route := range routeMapCell.GetRoute() {
			if route.RouteSubmissionStatus.Status != pogo.RouteSubmissionStatus_PUBLISHED {
				log.Warnf("Non published Route found in GetRoutesOutProto, status: %s", route.RouteSubmissionStatus.String())
				continue
			}
			decodeError := decoder.UpdateRouteRecordWithSharedRouteProto(dec.dbDetails, route)
			if decodeError != nil {
				if decodeErrors[route.Id] != true {
					decodeErrors[route.Id] = true
				}
				log.Errorf("Failed to decode route %s", decodeError)
			} else if decodeSuccesses[route.Id] != true {
				decodeSuccesses[route.Id] = true
			}
		}
	}

	return true, fmt.Sprintf(
		"Decoded %d routes, failed to decode %d routes, from %d cells",
		len(decodeSuccesses),
		len(decodeErrors),
		len(getRoutesOutProto.GetRouteMapCell()),
	)
}
