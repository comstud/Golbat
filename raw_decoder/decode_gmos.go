package raw_decoder

import (
	"context"
	"fmt"
	"golbat/decoder"
	"golbat/pogo"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

func isCellNotEmpty(mapCell *pogo.ClientMapCellProto) bool {
	return len(mapCell.Fort) > 0 || len(mapCell.WildPokemon) > 0 || len(mapCell.NearbyPokemon) > 0 || len(mapCell.CatchablePokemon) > 0
}

func cellContainsForts(mapCell *pogo.ClientMapCellProto) bool {
	return len(mapCell.Fort) > 0
}

func (dec *rawDecoder) decodeGMO(ctx context.Context, protoData *Proto) (bool, string) {
	scanParameters := getScanParameters(protoData)
	decodedGmo := &pogo.GetMapObjectsOutProto{}
	if err := proto.Unmarshal(protoData.ResponseProtoBytes(), decodedGmo); err != nil {
		dec.statsCollector.IncDecodeGMO("error", "parse")
		log.Errorf("Failed to parse %s", err)
		return true, fmt.Sprintf("Failed to parse %s", err)
	}

	if decodedGmo.Status != pogo.GetMapObjectsOutProto_SUCCESS {
		dec.statsCollector.IncDecodeGMO("error", "non_success")
		res := fmt.Sprintf(`GetMapObjectsOutProto: Ignored non-success value %d:%s`, decodedGmo.Status,
			pogo.GetMapObjectsOutProto_Status_name[int32(decodedGmo.Status)])
		return true, res
	}

	var newForts []decoder.RawFortData
	var newWildPokemon []decoder.RawWildPokemonData
	var newNearbyPokemon []decoder.RawNearbyPokemonData
	var newMapPokemon []decoder.RawMapPokemonData
	var newClientWeather []decoder.RawClientWeatherData
	var newMapCells []uint64
	var cellsToBeCleaned []uint64

	for _, mapCell := range decodedGmo.MapCell {
		if isCellNotEmpty(mapCell) {
			newMapCells = append(newMapCells, mapCell.S2CellId)
			if cellContainsForts(mapCell) {
				cellsToBeCleaned = append(cellsToBeCleaned, mapCell.S2CellId)
			}
		}
		timestampMs := uint64(mapCell.AsOfTimeMs)
		for _, fort := range mapCell.Fort {
			newForts = append(newForts, decoder.RawFortData{Cell: mapCell.S2CellId, Data: fort})

			if fort.ActivePokemon != nil {
				newMapPokemon = append(newMapPokemon, decoder.RawMapPokemonData{Cell: mapCell.S2CellId, Data: fort.ActivePokemon})
			}
		}
		for _, mon := range mapCell.WildPokemon {
			newWildPokemon = append(newWildPokemon, decoder.RawWildPokemonData{Cell: mapCell.S2CellId, Data: mon, Timestamp: timestampMs})
		}
		for _, mon := range mapCell.NearbyPokemon {
			newNearbyPokemon = append(newNearbyPokemon, decoder.RawNearbyPokemonData{Cell: mapCell.S2CellId, Data: mon})
		}
	}
	for _, clientWeather := range decodedGmo.ClientWeather {
		newClientWeather = append(newClientWeather, decoder.RawClientWeatherData{Cell: clientWeather.S2CellId, Data: clientWeather})
	}

	if scanParameters.ProcessGyms || scanParameters.ProcessPokestops {
		decoder.UpdateFortBatch(ctx, dec.dbDetails, scanParameters, newForts)
	}
	if scanParameters.ProcessPokemon {
		decoder.UpdatePokemonBatch(ctx, dec.dbDetails, scanParameters, newWildPokemon, newNearbyPokemon, newMapPokemon, protoData.Account)
	}
	if scanParameters.ProcessWeather {
		decoder.UpdateClientWeatherBatch(ctx, dec.dbDetails, newClientWeather)
	}
	if scanParameters.ProcessCells {
		decoder.UpdateClientMapS2CellBatch(ctx, dec.dbDetails, newMapCells)
		if scanParameters.ProcessGyms || scanParameters.ProcessPokestops {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				decoder.ClearRemovedForts(ctx, dec.dbDetails, cellsToBeCleaned)
			}()
		}
	}

	newFortsLen := len(newForts)
	newWildPokemonLen := len(newWildPokemon)
	newNearbyPokemonLen := len(newNearbyPokemon)
	newMapPokemonLen := len(newMapPokemon)
	newClientWeatherLen := len(newClientWeather)
	newMapCellsLen := len(newMapCells)

	dec.statsCollector.IncDecodeGMO("ok", "")
	dec.statsCollector.AddDecodeGMOType("fort", float64(newFortsLen))
	dec.statsCollector.AddDecodeGMOType("wild_pokemon", float64(newWildPokemonLen))
	dec.statsCollector.AddDecodeGMOType("nearby_pokemon", float64(newNearbyPokemonLen))
	dec.statsCollector.AddDecodeGMOType("map_pokemon", float64(newMapPokemonLen))
	dec.statsCollector.AddDecodeGMOType("weather", float64(newClientWeatherLen))
	dec.statsCollector.AddDecodeGMOType("cell", float64(newMapCellsLen))

	return true, fmt.Sprintf("%d cells containing %d forts %d mon %d nearby", newMapCellsLen, newFortsLen, newWildPokemonLen, newNearbyPokemonLen)
}
