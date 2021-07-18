# Installing

Run:

```
git clone https://github.com/arvidfm/addressbook
cd addressbook
go get addressbook
go build
./addressbook
```

This will launch an HTTP server at localhost:8080 which you can interact with using e.g. curl.
By default it will populate the database using the names listed in `names.csv`.
It will write logs to `addressbook.log` and write records to `addressbook.db`

To run tests, run:

```
go get -t addressbook
go test
```

# Endpoints

## GET /address/:id

Fetches the address record with the given ID.

```
$ curl "http://localhost:8080/address/1"
{"first_name":"Mark","id":1,"last_name":"Cairns","phone":"07508205146","success":true}
```

## GET /address

Fetches a list of matching address records.

Allowed query parameters:

* `sort`: Sorts the records according to the given field in ascending order. Allowed values are `first_name` and `last_name`.
* `limit`: How many records to return. Must be between 1 and 100.
* `search`: Filters names. Only records with first or last names with the given prefix will be returned.
* `last`: Used for pagination. Can be found in the `next` in the response.

For pagination, use the endpoint given in the `next` field in the response to request the next page.

```
$ curl "http://localhost:8080/address?search=Da&sort=first_name&limit=10"
{"addresses":[{"id":463,"first_name":"Cameron","last_name":"Davis","phone":"07195869798"},{"id":83,"first_name":"Daisy","last_name":"Whyte","phone":"02576554519"},{"id":124,"first_name":"Dale","last_name":"Lowe","phone":"06329504937"},{"id":246,"first_name":"Daniel","last_name":"Wightman","phone":null},{"id":365,"first_name":"Danielle","last_name":"Dalrymple","phone":"09937217774"},{"id":549,"first_name":"Darcy","last_name":"Gale","phone":"08992701900"},{"id":248,"first_name":"Darren","last_name":"Jolly","phone":null},{"id":107,"first_name":"Daryl","last_name":"Ewart","phone":"03133874883"},{"id":394,"first_name":"David","last_name":"Mcnicol","phone":"05867508341"},{"id":314,"first_name":"Dawn","last_name":"Dodd","phone":"07460071495"}],"next":"/address?last=314__Dawn\u0026limit=10\u0026search=Da\u0026sort=first_name","status":"success"}

$ curl "http://localhost:8080/address?last=314__Dawn&limit=10&search=Da&sort=first_name"
{"addresses":[{"id":211,"first_name":"Joshua","last_name":"Dalton","phone":null},{"id":476,"first_name":"Noah","last_name":"Dalrymple","phone":"01918137758"},{"id":467,"first_name":"Orla","last_name":"Darroch","phone":null}],"next":"/address?last=467__Orla\u0026limit=10\u0026search=Da\u0026sort=first_name","success":true}
```

# DELETE /address/:id

Remove a record from the database.

```
$ curl -X DELETE "http://localhost:8080/address/1"
{"success":true}
$ curl "http://localhost:8080/address/1"
{"error":"record not found"}
```

# POST /address

Add a new record to the database.

```
$ curl -X POST "http://localhost:8080/address" -d '{"first_name": "Jane", "last_name": "Doe", "phone": "07070707070"}'
{"id":575,"success":true}
```
