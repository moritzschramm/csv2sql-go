package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"io"
	"encoding/csv"
	"strings"
	"time"
	"log"
	"sync"
	"flag"
)

var (
	TABLENAME = ""
	FILENAME = ""
	DELIMITER = ','				// default delimiter for csv files
	MAX_SQL_CONNECTIONS = 145	// default max_connections of mysql is 150, 
)

// parse flags and command line arguments
func parseSysArgs() {

	
	table := flag.String("table", TABLENAME, "Name of MySQL database table.")
	delimiter := flag.String("d", string(DELIMITER), "Delimiter used in .csv file.")
	max_conns := flag.Int("max_conns", MAX_SQL_CONNECTIONS, "Maximum number of concurrent connections to database. Value depends on your MySQL configuration.")

	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		log.Fatal("Filename not specified. Only one file permitted. Use -h for help")
	}
	FILENAME = args[0]

	if *table == "" {	// if table name not set, guess tablename (use filename)

		if strings.HasSuffix(FILENAME, ".csv") {
			TABLENAME = FILENAME[:len(FILENAME) - len(".csv")]
		} else {
			TABLENAME = FILENAME
		}

	} else {
		TABLENAME = *table
	}
	

	DELIMITER = []rune(*delimiter)[0]
	MAX_SQL_CONNECTIONS = *max_conns
}

func main() {

	parseSysArgs()

	// --------------------------------------------------------------------------
	// prepare buffered file reader
	// --------------------------------------------------------------------------
	file, err := os.Open(FILENAME)
	if err != nil {
		log.Fatal(err.Error())
	}
	reader := csv.NewReader(file)
	reader.Comma = DELIMITER		// set custom comma for reader (default: ',')

	// --------------------------------------------------------------------------
	// database connection setup
	// --------------------------------------------------------------------------

	db, err := sql.Open("mysql", "homestead:secret@/homestead")
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	// check database connection
	err = db.Ping()
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	// set max idle connections
	db.SetMaxIdleConns(MAX_SQL_CONNECTIONS)
	defer db.Close()
	

	// --------------------------------------------------------------------------
	// read rows and insert into database
	// --------------------------------------------------------------------------

	start := time.Now()									// to measure execution time


	query := ""											// query statement
	callback 	:= make(chan int)						// callback channel for insert goroutines
	connections := 0									// number of concurrent connections
	insertions 	:= 0									// counts how many insertions have finished
	available 	:= make(chan bool, MAX_SQL_CONNECTIONS)	// buffered channel, holds number of available connections
	for i := 0; i < MAX_SQL_CONNECTIONS; i++ {
		available <- true
	}


	// start status logger
	startLogger(&insertions, &connections)		

	// start connection controller
	startConnectionController(&insertions, &connections, callback, available)	

	var wg sync.WaitGroup
	id := 1
	isFirstRow := true

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err.Error())
		}

		if isFirstRow {

			parseColumns(record, &query)
			isFirstRow = false

		} else if <-available {		// wait for available database connection

			connections += 1
			id += 1
			wg.Add(1)
			go insert(id, query, db, callback, &connections, &wg, string2interface(record))
		}
	}

	wg.Wait()

	elapsed := time.Since(start)
	log.Printf("Status: %d insertions\n", insertions)
	log.Printf("Execution time: %s\n", elapsed)
}

// inserts data into database
func insert(id int, query string, db *sql.DB, callback chan<- int, conns *int, wg *sync.WaitGroup, args []interface{}) {

	// make a new statement for every insert, 
	// this is quite inefficient, but since all inserts are running concurrently,
	// it's still faster than using a single prepared statement and 
	// inserting the data sequentielly.
	// we have to close the statement after the routine terminates,
	// so that the connection to the database is released and can be reused
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer stmt.Close()

	_, err = stmt.Exec(args...)
	if err != nil {
		log.Printf("ID: %d (%d conns), %s\n", id, *conns, err.Error())
	}

	// finished inserting, send id over channel to signalize termination of routine
	callback <- id
	wg.Done()
}

// controls termination of program and number of connections to database
func startConnectionController(insertions, connections *int, callback <-chan int, available chan<- bool) {

	go func() { for {

		<-callback			// returns id of terminated routine

		*insertions += 1	// a routine terminated, increment counter
		*connections -= 1	// and unregister its connection

		available <- true	// make new connection available
	}}()
}

// print status update to console every second
func startLogger(insertions, connections *int) {

	go func() {
		c := time.Tick(time.Second)
		for {
			<-c
			log.Printf("Status: %d insertions, %d database connections\n", *insertions, *connections)
		}
	}()
}

// parse csv columns, create query statement
func parseColumns(columns []string, query *string) {

	*query = "INSERT INTO "+TABLENAME+" ("
	placeholder := "VALUES ("
	for i, c := range columns {
		if i == 0 {
			*query += c
			placeholder += "?"
		} else {
			*query += ", "+c
			placeholder += ", ?"
		}
	} 
	placeholder += ")"
	*query += ") " + placeholder
}

// convert []string to []interface{}
func string2interface(s []string) []interface{} {

	i := make([]interface{}, len(s))
	for k, v := range s {
		i[k] = v
	}
	return i
}