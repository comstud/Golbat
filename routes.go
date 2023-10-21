package main

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"golbat/config"
	"golbat/decoder"
	"golbat/device_tracker"
	"golbat/geo"
	"golbat/raw_decoder"
)

var rawProtoDecoder raw_decoder.RawDecoder
var deviceTracker device_tracker.DeviceTracker

func Raw(c *gin.Context) {
	var w http.ResponseWriter = c.Writer
	var r *http.Request = c.Request

	authHeader := r.Header.Get("Authorization")
	if config.Config.RawBearer != "" {
		if authHeader != "Bearer "+config.Config.RawBearer {
			statsCollector.IncRawRequests("error", "auth")
			log.Errorf("Raw: Incorrect authorisation received (%s)", authHeader)
			return
		}
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 5*1048576))
	if err != nil {
		statsCollector.IncRawRequests("error", "io_error")
		log.Errorf("Raw: Error (1) during HTTP receive %s", err)
		return
	}
	if err := r.Body.Close(); err != nil {
		statsCollector.IncRawRequests("error", "io_close_error")
		log.Errorf("Raw: Error (2) during HTTP receive %s", err)
		return
	}

	protoData, err := rawProtoDecoder.GetProtoDataFromHTTP(r.Header, body)
	if err != nil {
		statsCollector.IncRawRequests("error", "decode")
		userAgent := r.Header.Get("User-Agent")
		log.Infof("Raw: Data could not be decoded. From User agent %s - Received data %s, err: %s", userAgent, body, err)
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	// Process each proto in a packet in sequence, but in a go-routine
	go rawProtoDecoder.Decode(context.Background(), protoData)

	deviceTracker.UpdateDeviceLocation(protoData.Uuid, protoData.Lat(), protoData.Lon(), protoData.ScanContext)

	statsCollector.IncRawRequests("ok", "")
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	//if err := json.NewEncoder(w).Encode(t); err != nil {
	//	panic(err)
	//}
}

func AuthRequired() gin.HandlerFunc {
	return func(context *gin.Context) {
		if config.Config.ApiSecret != "" {
			authHeader := context.Request.Header.Get("X-Golbat-Secret")
			if authHeader != config.Config.ApiSecret {
				log.Errorf("Incorrect authorisation received (%s)", authHeader)
				context.String(http.StatusUnauthorized, "Unauthorised")
				context.Abort()
				return
			}
		}
		context.Next()
	}
}

func ClearQuests(c *gin.Context) {
	fence, err := geo.NormaliseFenceRequest(c)

	if err != nil {
		log.Warnf("POST /api/clear-quests/ Error during post area %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Debugf("Clear quests %+v", fence)
	startTime := time.Now()
	decoder.ClearQuestsWithinGeofence(ctx, dbDetails, fence)
	log.Infof("Clear quest took %s", time.Since(startTime))

	c.JSON(http.StatusAccepted, map[string]interface{}{
		"status": "ok",
	})
}

func ReloadGeojson(c *gin.Context) {
	decoder.ReloadGeofenceAndClearStats()

	c.JSON(http.StatusAccepted, map[string]interface{}{
		"status": "ok",
	})
}

func ReloadNests(c *gin.Context) {
	decoder.ReloadNestsAndClearStats(dbDetails)

	c.JSON(http.StatusAccepted, map[string]interface{}{
		"status": "ok",
	})
}

func PokemonScan(c *gin.Context) {
	var requestBody decoder.ApiPokemonScan

	if err := c.BindJSON(&requestBody); err != nil {
		log.Warnf("POST /api/pokemon/scan/ Error during post retrieve %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	res := decoder.GetPokemonInArea(requestBody)
	if res == nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusAccepted, res)
}

func PokemonScan2(c *gin.Context) {
	var requestBody decoder.ApiPokemonScan2

	if err := c.BindJSON(&requestBody); err != nil {
		log.Warnf("POST /api/pokemon/scan/ Error during post retrieve %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	res := decoder.GetPokemonInArea2(requestBody)
	if res == nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusAccepted, res)
}

func PokemonOne(c *gin.Context) {
	pokemonId, err := strconv.ParseUint(c.Param("pokemon_id"), 10, 64)
	if err != nil {
		log.Warnf("GET /api/pokemon/:pokemon_id/ Error during get pokemon %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	res := decoder.GetOnePokemon(uint64(pokemonId))

	if res != nil {
		c.JSON(http.StatusAccepted, map[string]interface{}{
			"lat": res.Lat,
			"lon": res.Lon,
		})
	} else {
		c.Status(http.StatusNotFound)
	}
}

func PokemonAvailable(c *gin.Context) {
	res := decoder.GetAvailablePokemon()
	c.JSON(http.StatusAccepted, res)
}

func PokemonSearch(c *gin.Context) {
	var requestBody decoder.ApiPokemonSearch

	if err := c.BindJSON(&requestBody); err != nil {
		log.Warnf("POST /api/search/ Error during post search %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	res, err := decoder.SearchPokemon(requestBody)
	if err != nil {
		log.Warnf("POST /api/search/ Error during post search %v", err)
		c.Status(http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusAccepted, res)
}

func GetQuestStatus(c *gin.Context) {
	fence, err := geo.NormaliseFenceRequest(c)

	if err != nil {
		log.Warnf("POST /api/quest-status/ Error during post area %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	questStatus := decoder.GetQuestStatusWithGeofence(dbDetails, fence)

	c.JSON(http.StatusOK, &questStatus)
}

// GetHealth provides unrestricted health status for monitoring tools
func GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func GetPokestopPositions(c *gin.Context) {
	fence, err := geo.NormaliseFenceRequest(c)
	if err != nil {
		log.Warnf("POST /api/pokestop-positions/ Error during post area %v %v", err, fence)
		c.Status(http.StatusInternalServerError)
		return
	}

	response, err := decoder.GetPokestopPositions(dbDetails, fence)
	if err != nil {
		log.Warnf("POST /api/pokestop-positions/ Error during post retrieve %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusAccepted, response)
}

func GetPokestop(c *gin.Context) {
	fortId := c.Param("fort_id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	pokestop, err := decoder.GetPokestopRecord(ctx, dbDetails, fortId)
	cancel()
	if err != nil {
		log.Warnf("GET /api/pokestop/id/:fort_id/ Error during post retrieve %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusAccepted, pokestop)
}

func GetDevices(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"devices": deviceTracker.GetAllDevices()})
}
