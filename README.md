# cvs2sql

Inserts CVS records into MySQL database

## Setup
There is a prepared `docker-compose.yml` file to make things easier. 
If you have docker and docker-compose installed, run
```
docker-compose up -d
```
to start the MySQL container. Data will be stored in `./data`.
If you haven't installed docker, you have to create your own MySQL instance

## Usage
```
go run main.go -d="," -table="<tablename>" -max_conns=145 <filename>
```
Whereas -d is the comma used in your .csv file
Usually you only need to run:
```
go run main.go <filename>
```
if you have created a table with the same name as the file

## Example
There is a sample .csv file, `sample01.csv` (from [SpatialKey](https://support.spatialkey.com/spatialkey-sample-csv-data/)).
To use it, first, make sure the MySQL instance is running and properly setup. 
You can use `db-sample-setup.sql` to create the required table with columns. Then run:
```
go run main.go sample01.csv
```
This should populate your database table with ~36,000 entries

## Under the hood
The program uses a buffered reader to read each record in the csv. 
To insert the record concurrently, the program launches a new goroutine and executes the sql query, 
but only if there is a database connection available (MySQL limits the number of connections to 150 by default)