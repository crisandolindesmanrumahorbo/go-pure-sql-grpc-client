package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"strconv"

	pb "github.com/rest-example/proto"
	_ "github.com/sijms/go-ora/v2"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"time"
)

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type Config struct {
	DBService  string `mapstructure:"DB_SERVICE"`
	DBUsername string `mapstructure:"DB_USERNAME"`
	DBServer   string `mapstructure:"DB_SERVER"`
	DBPort     string `mapstructure:"DB_PORT"`
	DBPassword string `mapstructure:"DB_PASSWORD"`
}

var logger *log.Logger

// var people []Person
var config *Config
var db *sql.DB
var client pb.UserServiceClient

// gRPC
var (
	addr = flag.String("addr", "localhost:50051", "the address connect to")
)

type User struct {
	FirstName string  `json:"firstname"`
	Age       uint8   `json:"age"`
	Address   Address `json:"address"`
}
type Address struct {
	City    string `json:"city"`
	ZipCode string `json:"zipCode"`
}

func NewDB(dbmock *sql.DB) {
	db = dbmock
}

func init() {
	initLogger()
	// initEnv()
	// initSqlDBWithPureDriver()
	// // defer func() {
	// // 	err := db.Close()
	// // 	if err != nil {
	// // 		fmt.Println("Can't close connection: ", err)
	// // 	}
	// // }()
	// // sqlExampleOperations()
	// createTable()
}

func loadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()
	err = viper.ReadInConfig()

	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		return
	}
	return
}

func closeDBConnection() {
	err := db.Close()
	if err != nil {
		fmt.Println("Can't close connection: ", err)
	}
}

func sqlQueryByUsername(username string) (*Person, error) {
	const queryTable = "SELECT username, age FROM DEMO.persons WHERE username = :username"
	var queryUsername string
	var queryAge int
	row := db.QueryRow(queryTable, username)
	err := row.Scan(&queryUsername, &queryAge)
	if err == sql.ErrNoRows {
		return nil, errors.New("not found")
	}
	if err != nil {
		return nil, displayError("query by name", err)
	}
	logger.Printf("Age of %v is %v", queryUsername, queryAge)
	return &Person{
		Name: queryUsername,
		Age:  queryAge,
	}, nil
}

func sqlQueryPersons() (result []Person, err error) {
	var name string
	var age int
	const queryPersons = "SELECT username, age FROM DEMO.persons"
	rows, err := db.Query(queryPersons)
	defer rows.Close()
	if err != nil {
		return nil, displayError("query persons", err)
	}
	for rows.Next() {
		err := rows.Scan(&name, &age)
		if err != nil {
			return result, displayError("query persons scan", err)
		}
		person := Person{
			Name: name,
			Age:  age,
		}
		result = append(result, person)
	}
	return result, nil
}

func sqlInsert(person Person) (id int64, err error) {
	const insertTable = "INSERT INTO DEMO.persons (username, age) VALUES (:username, :age) RETURNING id into :id"
	_, errInsert := db.Exec(insertTable, sql.Named("username", person.Name), sql.Named("age", person.Age), sql.Named("id", sql.Out{Dest: &id}))
	if errInsert != nil {
		return 0, displayError("insert table", errInsert)
	}
	// id, err := result.LastInsertId()
	// if err != nil {
	// 	displayError("insert table", err)
	// }
	// logger.Printf("id inserted %v", id)
	return id, nil
}

func displayError(msg string, err error) error {
	logger.Panicf("error when %v : %v", msg, err.Error())
	return errors.New(err.Error())
}

func createTable() {
	const createTable = "CREATE TABLE DEMO.persons (id NUMBER GENERATED BY DEFAULT AS IDENTITY, PRIMARY KEY(id), username VARCHAR2(50), age NUMBER(3), creation_time TIMESTAMP DEFAULT SYSTIMESTAMP)"
	const dropTable = "DROP TABLE DEMO.persons PURGE"

	db.Exec(dropTable)
	// defer db.Exec(dropTable)
	_, errCreate := db.Exec(createTable)
	if errCreate != nil {
		displayError("create table", errCreate)
	}
}

func sqlExampleOperations() {
	var queryResultColumnOne string
	row := db.QueryRow("SELECT to_char(systimestamp,'HH24:MI:SS') FROM dual")
	err := row.Scan(&queryResultColumnOne)
	if err != nil {
		panic(fmt.Errorf("error scanning query result from database into target variable: %w", err))
	}
	fmt.Println("The time in the database ", queryResultColumnOne)
}

func main() {
	// flag.Parse()
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatalf("Did not connect: %v", err)
	}
	defer conn.Close()
	client = pb.NewUserServiceClient(conn)
	// people = []Person{
	// 	{Name: "cris", Age: 26},
	// 	{Name: "dolin", Age: 26},
	// 	{Name: "desman", Age: 26},
	// 	{Name: "rumbo", Age: 26},
	// }
	http.HandleFunc("/person/", middleware(handler))
	http.HandleFunc("/hello", middleware(helloHandler))
	http.HandleFunc("/users/", middleware(userHandler))
	log.Fatal(http.ListenAndServe(":8082", nil))

	//TODO gracefull shutdown listen apps running or not, then remove db connection, http servere
	// defer db.Close()

}

func middleware(handler func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		start := time.Now()
		logger.Printf("Request %v %v", r.Method, r.URL)
		handler(w, r)
		elapsed := time.Since(start).Microseconds()
		logger.Printf("Response time %v μs", elapsed)
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello World!\n")
}

func handler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/person/")
	switch r.Method {
	case "GET":
		if name == "" {
			getPeople(w)
			return
		}
		getPersonByName(w, name)
		return
	case "POST":
		addPerson(w, r)
	}
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/users/")
	switch r.Method {
	case "GET":
		if id != "" {
			getUser(w, id)
			return
		}
	case "POST":
		addUser(w, r)
	}
}

func getPeople(w http.ResponseWriter) {
	people, err := sqlQueryPersons()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(people)
}

func getPersonByName(w http.ResponseWriter, name string) {
	var person *Person
	// for i, val := range people {
	// 	log.Print(i, val)
	// 	if val.Name == name {
	// 		person = &val
	// 		break
	// 	}
	// }
	person, err := sqlQueryByUsername(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	logger.Print(person, name)
	json.NewEncoder(w).Encode(person)
}

func addPerson(w http.ResponseWriter, r *http.Request) {
	var person Person
	err := json.NewDecoder(r.Body).Decode(&person)
	logger.Printf("body %v", person)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// people = append(people, person)
	personId, err := sqlInsert(person)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(personId)
}

func initSqlDBWithPureDriver() {
	connectionString := "oracle://" + config.DBUsername + ":" + config.DBPassword + "@" + config.DBServer + ":" + config.DBPort + "/" + config.DBService
	instanceDB, err := sql.Open("oracle", connectionString)
	if err != nil {
		panic(fmt.Errorf("error in sql.Open: %w", err))
	}
	err = instanceDB.Ping()
	if err != nil {
		panic(fmt.Errorf("error pinging db: %w", err))
	}
	db = instanceDB
}

func initEnv() {
	configLoad, err := loadConfig(".")
	config = &configLoad
	if err != nil {
		logger.Fatalf("can't load environment app.env: %v", err)
	}
}

func initLogger() {
	const (
		YYYYMMDD  = "2006-01-02"
		HHMMSS12h = "3:04:05 PM"
	)
	logger = log.New(os.Stdout, time.Now().UTC().Format(YYYYMMDD+" "+HHMMSS12h)+": ", log.Lshortfile)
}

func getUser(w http.ResponseWriter, id string) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res, err := client.GetUser(context.Background(), &pb.ReadUserRequest{Id: int64(idInt)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(res.User)
}

func addUser(w http.ResponseWriter, r *http.Request) {
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reqAddress := &pb.Address{
		City:    user.Address.City,
		ZipCode: user.Address.ZipCode,
	}
	reqUser := &pb.User{
		Address:   reqAddress,
		Firstname: user.FirstName,
		Age:       uint32(user.Age),
	}
	res, err := client.CreateUser(context.Background(), &pb.CreateUserRequest{User: reqUser})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(res.Id)
}
