package stats_collector

import (
	"golbat/geo"

	"gopkg.in/guregu/null.v4"
)

var _ StatsCollector = (*noopCollector)(nil)

type noopCollector struct {
}

func (col *noopCollector) IncRawRequests(string, string)                                 {}
func (col *noopCollector) IncDecodeMethods(string, string, string)                       {}
func (col *noopCollector) IncDecodeFortDetails(string, string)                           {}
func (col *noopCollector) IncDecodeGetMapForts(string, string)                           {}
func (col *noopCollector) IncDecodeGetGymInfo(string, string)                            {}
func (col *noopCollector) IncDecodeEncounter(string, string, []geo.AreaName)             {}
func (col *noopCollector) IncDecodeDiskEncounter(string, string, []geo.AreaName)         {}
func (col *noopCollector) IncDecodeEncounterType(string, string, string, []geo.AreaName) {}
func (col *noopCollector) IncDecodeQuest(string, string, []geo.AreaName)                 {}
func (col *noopCollector) IncDecodeSocialActionWithRequest(string, string)               {}
func (col *noopCollector) IncDecodeGetFriendDetails(string, string)                      {}
func (col *noopCollector) IncDecodeSearchPlayer(string, string)                          {}
func (col *noopCollector) IncDecodeGMO(string, string)                                   {}
func (col *noopCollector) AddDecodeGMOType(string, float64)                              {}
func (col *noopCollector) IncDecodeStartIncident(string, string)                         {}
func (col *noopCollector) IncDecodeOpenInvasion(string, string)                          {}
func (col *noopCollector) AddPokemonStatsResetCount(string, float64)                     {}
func (col *noopCollector) IncPokemonCountNew(string)                                     {}
func (col *noopCollector) IncPokemonCountIv(string)                                      {}
func (col *noopCollector) IncPokemonCountShiny(string, string)                           {}
func (col *noopCollector) IncPokemonCountNonShiny(string, string)                        {}
func (col *noopCollector) IncPokemonCountShundo(string)                                  {}
func (col *noopCollector) IncPokemonCountSnundo(string)                                  {}
func (col *noopCollector) IncPokemonCountHundo(string)                                   {}
func (col *noopCollector) IncPokemonCountNundo(string)                                   {}
func (col *noopCollector) UpdateVerifiedTtl(geo.AreaName, null.String, null.Int)         {}
func (col *noopCollector) UpdateRaidCount([]geo.AreaName, int64)                         {}
func (col *noopCollector) UpdateFortCount([]geo.AreaName, string, string)                {}
func (col *noopCollector) UpdateIncidentCount([]geo.AreaName)                            {}

func NewNoopStatsCollector() StatsCollector {
	return &noopCollector{}
}
