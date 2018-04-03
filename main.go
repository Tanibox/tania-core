package main

import (
	"database/sql"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/Tanibox/tania-server/src/eventbus"
	"github.com/asaskevich/EventBus"

	"github.com/go-sql-driver/mysql"

	"github.com/Tanibox/tania-server/config"
	assetsserver "github.com/Tanibox/tania-server/src/assets/server"
	assetsstorage "github.com/Tanibox/tania-server/src/assets/storage"
	growthserver "github.com/Tanibox/tania-server/src/growth/server"
	growthstorage "github.com/Tanibox/tania-server/src/growth/storage"
	locationserver "github.com/Tanibox/tania-server/src/location/server"
	tasksserver "github.com/Tanibox/tania-server/src/tasks/server"
	taskstorage "github.com/Tanibox/tania-server/src/tasks/storage"
	userserver "github.com/Tanibox/tania-server/src/user/server"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	_ "github.com/mattn/go-sqlite3"
	"github.com/paked/configure"
	uuid "github.com/satori/go.uuid"
)

func init() {
	initConfig()
}

func main() {
	e := echo.New()

	// Initialize DB.
	log.Print("Using " + *config.Config.TaniaPersistenceEngine + " persistance engine")

	// InMemory DB will always be initialized.
	inMem := initInMemory()

	var db *sql.DB
	switch *config.Config.TaniaPersistenceEngine {
	case config.DB_SQLITE:
		db = initSqlite()
	case config.DB_MYSQL:
		db = initMysql()
	}

	// Initialize Event Bus
	bus := eventbus.NewSimpleEventBus(EventBus.New())

	// Initialize Server
	farmServer, err := assetsserver.NewFarmServer(
		db,
		inMem.farmEventStorage,
		inMem.farmReadStorage,
		inMem.areaEventStorage,
		inMem.areaReadStorage,
		inMem.reservoirEventStorage,
		inMem.reservoirReadStorage,
		inMem.materialEventStorage,
		inMem.materialReadStorage,
		inMem.cropReadStorage,
		bus,
	)
	if err != nil {
		e.Logger.Fatal(err)
	}

	taskServer, err := tasksserver.NewTaskServer(
		db,
		bus,
		inMem.cropReadStorage,
		inMem.areaReadStorage,
		inMem.materialReadStorage,
		inMem.reservoirReadStorage,
		inMem.taskEventStorage,
		inMem.taskReadStorage,
	)
	if err != nil {
		e.Logger.Fatal(err)
	}

	growthServer, err := growthserver.NewGrowthServer(
		db,
		bus,
		inMem.cropEventStorage,
		inMem.cropReadStorage,
		inMem.cropActivityStorage,
		inMem.areaReadStorage,
		inMem.materialReadStorage,
		inMem.farmReadStorage,
		inMem.taskReadStorage,
	)
	if err != nil {
		e.Logger.Fatal(err)
	}

	userServer, err := userserver.NewUserServer(db, bus)
	if err != nil {
		e.Logger.Fatal(err)
	}

	authServer, err := userserver.NewAuthServer(db, bus)
	if err != nil {
		e.Logger.Fatal(err)
	}

	locationServer, err := locationserver.NewLocationServer()
	if err != nil {
		e.Logger.Fatal(err)
	}

	// Initialize user
	err = initUser(authServer)

	// Initialize Echo Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(headerNoCache)

	APIMiddlewares := []echo.MiddlewareFunc{}
	if !*config.Config.DemoMode {
		APIMiddlewares = append(APIMiddlewares, tokenValidationWithConfig(db))
	}

	// HTTP routing
	API := e.Group("api")
	API.Use(middleware.CORS())

	// AuthServer is used for endpoint that doesn't need authentication checking
	authGroup := API.Group("/")
	authServer.Mount(authGroup)

	locationGroup := API.Group("/locations", APIMiddlewares...)
	locationServer.Mount(locationGroup)

	farmGroup := API.Group("/farms", APIMiddlewares...)
	farmServer.Mount(farmGroup)
	growthServer.Mount(farmGroup)

	taskGroup := API.Group("/tasks", APIMiddlewares...)
	taskServer.Mount(taskGroup)

	userGroup := API.Group("/user", APIMiddlewares...)
	userServer.Mount(userGroup)

	e.Static("/", "public")

	// Start Server
	e.Logger.Fatal(e.Start(":8080"))
}

/*
	Example setting and usage of configure package:

	// main.initConfig()
	configuration := config.Configuration{
		// this will be filled from environment variables
		DBPassword: conf.String("TANIA_DB_PASSWORD", "123456", "Description"),

		// this will be filled from flags (ie ./tania-server --port=9000)
		Port: conf.String("port", "3000", "Description"),

		// this will be filled from conf.json
		UploadPath: conf.String("UploadPath", "/home/tania/uploads", "Description"),
	}

	// config.Configuration struct
	type Configuration struct {
		DBPassword 		*string
		Port 			*string
		UploadPath 		*string
	}

	// Usage. config.Config can be called globally
	fmt.Println(*config.Config.DBPassword)
	fmt.Println(*config.Config.Port)
	fmt.Println(*config.Config.UploadPath)

*/
func initConfig() {
	conf := configure.New()

	configuration := config.Configuration{
		UploadPathArea:         conf.String("upload_path_area", "tania-uploads/area", "Upload path for the Area photo"),
		UploadPathCrop:         conf.String("upload_path_crop", "tania-uploads/crop", "Upload path for the Crop photo"),
		DemoMode:               conf.Bool("demo_mode", true, "Switch for the demo mode"),
		TaniaPersistenceEngine: conf.String("tania_persistence_engine", "sqlite", "The persistance engine of Tania. Options are inmemory, sqlite, inmemory"),
		SqlitePath:             conf.String("sqlite_path", "tania.db", "Path of sqlite file db"),
		MysqlHost:              conf.String("mysql_host", "127.0.0.1", "Mysql Host"),
		MysqlPort:              conf.String("mysql_port", "3306", "Mysql Port"),
		MysqlDbname:            conf.String("mysql_dbname", "tania", "Mysql DBName"),
		MysqlUsername:          conf.String("mysql_username", "root", "Mysql username"),
		MysqlPassword:          conf.String("mysql_password", "root", "Mysql password"),
		RedirectURI:            conf.String("redirect_uri", "http://localhost:8080/oauth2_implicit_callback", "URI for redirection after authorization server grants access token"),
		ClientID:               conf.String("client_id", "f0ece679-3f53-463e-b624-73e83049d6ac", "OAuth2 Implicit Grant Client ID for frontend"),
	}

	// This config will read the first configuration.
	// If it doesn't find the key, then it go to the next configuration.
	conf.Use(configure.NewEnvironment())
	conf.Use(configure.NewFlag())

	if _, err := os.Stat("conf.json"); err == nil {
		log.Print("Using 'conf.json' configuration")
		conf.Use(configure.NewJSONFromFile("conf.json"))
	}

	conf.Parse()

	config.Config = configuration
}

func initUser(authServer *userserver.AuthServer) error {
	defaultUsername := "tania"
	defaultPassword := "tania"

	_, _, err := authServer.RegisterNewUser(defaultUsername, defaultPassword, defaultPassword)
	if err != nil {
		log.Print("User ", defaultUsername, " has already created")
		return err
	}

	log.Print("User created with default username and password")

	return nil
}

// MIDDLEWARES

func headerNoCache(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // HTTP 1.1.
		c.Response().Header().Set("Pragma", "no-cache")                                   // HTTP 1.0.
		c.Response().Header().Set("Expires", "0")                                         // Proxies.
		return next(c)
	}
}

type InMemory struct {
	farmEventStorage      *assetsstorage.FarmEventStorage
	farmReadStorage       *assetsstorage.FarmReadStorage
	areaEventStorage      *assetsstorage.AreaEventStorage
	areaReadStorage       *assetsstorage.AreaReadStorage
	reservoirEventStorage *assetsstorage.ReservoirEventStorage
	reservoirReadStorage  *assetsstorage.ReservoirReadStorage
	materialEventStorage  *assetsstorage.MaterialEventStorage
	materialReadStorage   *assetsstorage.MaterialReadStorage
	cropEventStorage      *growthstorage.CropEventStorage
	cropReadStorage       *growthstorage.CropReadStorage
	cropActivityStorage   *growthstorage.CropActivityStorage
	taskEventStorage      *taskstorage.TaskEventStorage
	taskReadStorage       *taskstorage.TaskReadStorage
}

func initInMemory() *InMemory {
	return &InMemory{
		farmEventStorage: assetsstorage.CreateFarmEventStorage(),
		farmReadStorage:  assetsstorage.CreateFarmReadStorage(),

		areaEventStorage: assetsstorage.CreateAreaEventStorage(),
		areaReadStorage:  assetsstorage.CreateAreaReadStorage(),

		reservoirEventStorage: assetsstorage.CreateReservoirEventStorage(),
		reservoirReadStorage:  assetsstorage.CreateReservoirReadStorage(),

		materialEventStorage: assetsstorage.CreateMaterialEventStorage(),
		materialReadStorage:  assetsstorage.CreateMaterialReadStorage(),

		cropEventStorage:    growthstorage.CreateCropEventStorage(),
		cropReadStorage:     growthstorage.CreateCropReadStorage(),
		cropActivityStorage: growthstorage.CreateCropActivityStorage(),

		taskEventStorage: taskstorage.CreateTaskEventStorage(),
		taskReadStorage:  taskstorage.CreateTaskReadStorage(),
	}
}

func initMysql() *sql.DB {
	host := *config.Config.MysqlHost
	port := *config.Config.MysqlPort
	dbname := *config.Config.MysqlDbname
	user := *config.Config.MysqlUsername
	pwd := *config.Config.MysqlPassword

	dsn := user + ":" + pwd + "@(" + host + ":" + port + ")/" + dbname + "?parseTime=true&clientFoundRows=true"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}

	log.Print("Using MySQL at ", host, ":", port, "/", dbname)

	ddl, err := ioutil.ReadFile("db/mysql/ddl.sql")
	if err != nil {
		panic(err)
	}
	sqls := string(ddl)

	// We need to split the DDL query by `;` and execute it one by one.
	// Because sql.DB.Exec() from mysql driver cannot executes multiple query at once
	// and it will give weird syntax error messages.
	splitted := strings.Split(sqls, ";")

	for _, v := range splitted {
		trimmed := strings.TrimSpace(v)

		if len(trimmed) > 0 {
			_, err = db.Exec(v)

			if err != nil {
				me, ok := err.(*mysql.MySQLError)
				if !ok {
					panic("Error executing DDL query")
				}

				// http://dev.mysql.com/doc/refman/5.7/en/error-messages-server.html
				// We will skip error duplicate key name in database (code: 1061),
				// because CREATE INDEX doesn't have IF NOT EXISTS clause,
				// otherwise we will stop the loop and print the error
				if me.Number == 1061 {

				} else {
					log.Print(err)
					return db
				}
			}
		}
	}

	log.Print("DDL file executed")

	return db
}

func initSqlite() *sql.DB {
	if _, err := os.Stat(*config.Config.SqlitePath); os.IsNotExist(err) {
		log.Print("Creating database file ", *config.Config.SqlitePath)
	}

	db, err := sql.Open("sqlite3", *config.Config.SqlitePath)
	if err != nil {
		panic(err)
	}

	log.Print("Using SQLite at ", *config.Config.SqlitePath)

	log.Print("Executing DDL file for ", *config.Config.SqlitePath)

	ddl, err := ioutil.ReadFile("db/sqlite/ddl.sql")
	if err != nil {
		panic(err)
	}
	sql := string(ddl)

	_, err = db.Exec(sql)
	if err != nil {
		panic(err)
	}

	log.Print("DDL file executed")

	return db
}

func tokenValidationWithConfig(db *sql.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authorization := c.Request().Header.Get("Authorization")

			if authorization == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"data": "Unauthorized"})
			}

			splitted := strings.Split(authorization, " ")
			if len(splitted) <= 1 {
				return c.JSON(http.StatusUnauthorized, map[string]string{"data": "Unauthorized"})
			}

			uid := ""
			err := db.QueryRow(`SELECT USER_UID
				FROM USER_AUTH WHERE ACCESS_TOKEN = ?`, splitted[1]).Scan(&uid)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"data": "Unauthorized"})
			}

			userUID, err := uuid.FromString(uid)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]error{"data": err})
			}

			c.Set("USER_UID", userUID)

			return next(c)
		}
	}
}
