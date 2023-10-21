package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	ginlogrus "github.com/toorop/gin-logrus"
	"google.golang.org/grpc"

	"golbat/config"
	db2 "golbat/db"
	"golbat/decoder"
	"golbat/device_tracker"
	"golbat/external"
	pb "golbat/grpc"
	"golbat/raw_decoder"
	"golbat/stats_collector"
	"golbat/webhooks"
)

var db *sqlx.DB
var dbDetails db2.DbDetails
var statsCollector stats_collector.StatsCollector

func main() {
	var wg sync.WaitGroup
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	wg.Add(1)
	go func() {
		defer wg.Done()
		watchForShutdown(ctx, cancelFn)
	}()

	cfg, err := config.ReadConfig()
	if err != nil {
		panic(err)
	}

	logLevel := log.InfoLevel

	if cfg.Logging.Debug == true {
		logLevel = log.DebugLevel
	}
	SetupLogger(
		logLevel,
		cfg.Logging.SaveLogs,
		cfg.Logging.MaxSize,
		cfg.Logging.MaxAge,
		cfg.Logging.MaxBackups,
		cfg.Logging.Compress,
	)

	log.Infof("Golbat starting")

	// Both Sentry & Pyroscope are optional and off by default. Read more:
	// https://docs.sentry.io/platforms/go
	// https://pyroscope.io/docs/golang
	external.InitSentry()
	external.InitPyroscope()

	webhooksSender, err := webhooks.NewWebhooksSender(cfg)
	if err != nil {
		log.Fatalf("failed to setup webhooks sender: %s", err)
	}
	decoder.SetWebhooksSender(webhooksSender)

	// Capture connection properties.
	mysqlConfig := mysql.Config{
		User:                 cfg.Database.User,     //"root",     //os.Getenv("DBUSER"),
		Passwd:               cfg.Database.Password, //"transmit", //os.Getenv("DBPASS"),
		Net:                  "tcp",
		Addr:                 cfg.Database.Addr,
		DBName:               cfg.Database.Db,
		AllowNativePasswords: true,
	}

	dbConnectionString := mysqlConfig.FormatDSN()
	driver := "mysql"

	log.Infof("Starting migration")

	m, err := migrate.New(
		"file://sql",
		driver+"://"+dbConnectionString+"&multiStatements=true")
	if err != nil {
		log.Fatal(err)
		return
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		log.Fatal(err)
		return
	}

	log.Infof("Opening database for processing, max pool = %d", cfg.Database.MaxPool)

	// Get a database handle.

	db, err = sqlx.Open(driver, dbConnectionString)
	if err != nil {
		log.Fatal(err)
		return
	}

	db.SetConnMaxLifetime(time.Minute * 3) // Recommended by go mysql driver
	db.SetMaxOpenConns(cfg.Database.MaxPool)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(time.Minute)

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
		return
	}
	log.Infoln("Connected to database")

	decoder.SetKojiUrl(cfg.Koji.Url, cfg.Koji.BearerToken)

	//if cfg.LegacyInMemory {
	//	// Initialise in memory db
	//	inMemoryDb, err = sqlx.Open("sqlite3", ":memory:")
	//	if err != nil {
	//		log.Fatal(err)
	//		return
	//	}
	//
	//	inMemoryDb.SetMaxOpenConns(1)
	//
	//	pingErr = inMemoryDb.Ping()
	//	if pingErr != nil {
	//		log.Fatal(pingErr)
	//		return
	//	}
	//
	//	// Create database
	//	content, fileErr := ioutil.ReadFile("sql/sqlite/create.sql")
	//
	//	if fileErr != nil {
	//		log.Fatal(err)
	//	}
	//
	//	inMemoryDb.MustExec(string(content))
	//
	//	dbDetails = db2.DbDetails{
	//		PokemonDb:       inMemoryDb,
	//		UsePokemonCache: false,
	//		GeneralDb:       db,
	//	}
	//} else {
	dbDetails = db2.DbDetails{
		PokemonDb:       db,
		UsePokemonCache: true,
		GeneralDb:       db,
	}
	//}

	// Create the web server.
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	if cfg.Logging.Debug {
		r.Use(ginlogrus.Logger(log.StandardLogger()))
	} else {
		r.Use(gin.Recovery())
	}

	// choose the statsCollector we will use.
	statsCollector = stats_collector.GetStatsCollector(cfg, r)
	// tell the decoder the stats collector to use
	decoder.SetStatsCollector(statsCollector)
	db2.SetStatsCollector(statsCollector)

	// collect live stats when prometheus and liveStats are enabled
	if cfg.Prometheus.Enabled && cfg.Prometheus.LiveStats {
		go db2.PromLiveStatsUpdater(dbDetails, cfg.Prometheus.LiveStatsSleep)
	}

	decoder.InitialiseOhbem()
	decoder.LoadStatsGeofences()
	decoder.LoadNests(dbDetails)

	deviceTracker = device_tracker.NewDeviceTracker(cfg.Cleanup.DeviceHours)
	wg.Add(1)
	go func() {
		defer cancelFn()
		defer wg.Done()
		deviceTracker.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer cancelFn()
		defer wg.Done()

		err := webhooksSender.Run(ctx)
		if err != nil {
			log.Errorf("failed to start webhooks sender: %s", err)
		}
	}()

	log.Infoln("Golbat started")

	StartDbUsageStatsLogger(db)
	decoder.StartStatsWriter(db)

	if cfg.Tuning.ExtendedTimeout {
		log.Info("Extended timeout enabled")
	}

	if cfg.Cleanup.Pokemon == true && !cfg.PokemonMemoryOnly {
		StartDatabaseArchiver(db)
	}

	if cfg.Cleanup.Incidents == true {
		StartIncidentExpiry(db)
	}

	if cfg.Cleanup.Quests == true {
		StartQuestExpiry(db)
	}

	if cfg.Cleanup.Stats == true {
		StartStatsExpiry(db)
	}

	if cfg.TestFortInMemory {
		go decoder.LoadAllPokestops(dbDetails)
		go decoder.LoadAllGyms(dbDetails)
	}

	// Start the GRPC receiver

	if cfg.GrpcPort > 0 {
		log.Infof("Starting GRPC server on port %d", cfg.GrpcPort)
		go func() {
			lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GrpcPort))
			if err != nil {
				log.Fatalf("failed to listen: %v", err)
			}
			s := grpc.NewServer()
			pb.RegisterRawProtoServer(s, &grpcRawServer{})
			pb.RegisterPokemonServer(s, &grpcPokemonServer{})
			log.Printf("grpc server listening at %v", lis.Addr())
			if err := s.Serve(lis); err != nil {
				log.Fatalf("failed to serve: %v", err)
			}
		}()
	}

	{
		timeout := 5 * time.Second
		if cfg.Tuning.ExtendedTimeout {
			timeout = 30 * time.Second
		}
		rawProtoDecoder = raw_decoder.NewRawDecoder(timeout, dbDetails, statsCollector)
	}

	r.POST("/raw", Raw)
	r.GET("/health", GetHealth)

	apiGroup := r.Group("/api", AuthRequired())
	apiGroup.GET("/health", GetHealth)
	apiGroup.POST("/clear-quests", ClearQuests)
	apiGroup.POST("/quest-status", GetQuestStatus)
	apiGroup.POST("/pokestop-positions", GetPokestopPositions)
	apiGroup.GET("/pokestop/id/:fort_id", GetPokestop)
	apiGroup.POST("/reload-geojson", ReloadGeojson)
	apiGroup.GET("/reload-geojson", ReloadGeojson)
	apiGroup.POST("/reload-nests", ReloadNests)
	apiGroup.GET("/reload-nests", ReloadNests)

	apiGroup.GET("/pokemon/id/:pokemon_id", PokemonOne)
	apiGroup.GET("/pokemon/available", PokemonAvailable)
	apiGroup.POST("/pokemon/scan", PokemonScan)
	apiGroup.POST("/pokemon/v2/scan", PokemonScan2)
	apiGroup.POST("/pokemon/search", PokemonSearch)

	apiGroup.GET("/devices/all", GetDevices)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	// Start the server in a goroutine, as it will block until told to shutdown.
	wg.Add(1)
	go func() {
		defer cancelFn()
		defer wg.Done()

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("Failed to listen and start http server: %s", err)
		}
	}()

	// wait for shutdown to be signaled in some way. This can be from a failure
	// to start the webhook sender, failure to start the http server, and/or
	// watchForShutdown() saying it is time to shutdown. (watchForShutdown() on unix
	// waits for a SIGINT or SIGTERM)
	<-ctx.Done()

	log.Info("Starting shutdown...")

	// So now we attempt to shutdown the http server, telling it to wait for open requests to
	// finish for 5 seconds before just pulling the plug.
	shutdownCtx, shutdownCancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancelFn()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		if err == context.DeadlineExceeded {
			log.Warn("Graceful shutdown timed out, exiting.")
		} else {
			log.Errorf("Error during http server shutdown: %s", err)
		}
	}

	// wait for other started goroutines to cleanup and exit before we flush the
	// webhooks and exit the program.
	log.Info("http server is shutdown, waiting for other go routines to exit...")
	wg.Wait()

	log.Info("go routines have exited, flushing webhooks now...")
	webhooksSender.Flush()

	log.Info("Golbat exiting!")
}
