package stats_collector

import (
	"golbat/geo"

	"gopkg.in/guregu/null.v4"
)

type StatsCollector interface {
	IncRawRequests(status, message string)
	IncDecodeMethods(status, message, method string)
	IncDecodeFortDetails(status, message string)
	IncDecodeGetMapForts(status, message string)
	IncDecodeGetGymInfo(status, message string)
	IncDecodeEncounter(status, messages string)
	IncDecodeDiskEncounter(status, message string)
	IncDecodeQuest(status, message string)
	IncDecodeSocialActionWithRequest(status, message string)
	IncDecodeGetFriendDetails(status, message string)
	IncDecodeSearchPlayer(status, message string)
	IncDecodeGMO(status, message string)
	AddDecodeGMOType(typ string, value float64)
	IncDecodeStartIncident(status, message string)
	IncDecodeOpenInvasion(status, message string)
	AddPokemonStatsResetCount(area string, val float64)
	IncPokemonCountNew(area string)
	IncPokemonCountIv(area string)
	IncPokemonCountShiny(area string, pokemonId string)
	IncPokemonCountNonShiny(area string, pokemonId string)
	IncPokemonCountShundo(area string)
	IncPokemonCountSnundo(area string)
	IncPokemonCountHundo(area string)
	IncPokemonCountNundo(area string)
	UpdateVerifiedTtl(area geo.AreaName, seenType null.String, expireTimestamp null.Int)
	UpdateRaidCount(areas []geo.AreaName, raidLevel int64)
	UpdateFortCount(areas []geo.AreaName, fortType string, changeType string)
	UpdateIncidentCount(areas []geo.AreaName)
}
