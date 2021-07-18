package main

import (
	"encoding/csv"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Addresses struct {
	ID        uint    `gorm:"primaryKey" json:"id"`
	FirstName string  `gorm:"index;not null" json:"first_name"`
	LastName  string  `gorm:"index;not null" json:"last_name"`
	Phone     *string `json:"phone"`
}

func populate(db *gorm.DB) {
	// populate with default data
	file, err := os.Open("names.csv")
	if err != nil {
		fmt.Println("warning: names.csv not found; will not populate with default data")
	} else {
		csvreader := csv.NewReader(file)
		addresses := make([]Addresses, 0)
		for {
			record, err := csvreader.Read()
			if err == io.EOF {
				break
			}
			var phone *string
			if len(record[2]) > 0 {
				phone = &record[2]
			}
			address := Addresses{FirstName: record[0], LastName: record[1], Phone: phone}
			addresses = append(addresses, address)
		}
		db.Create(&addresses)
	}
}

func initialize_database(filename string) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(filename), &gorm.Config{ /*Logger: logger.Default.LogMode(logger.Info)*/ })
	if err != nil {
		panic("failed to connect to database")
	}

	db.AutoMigrate(&Addresses{})

	var count int64
	db.Model(&Addresses{}).Count(&count)

	if count == 0 {
		populate(db)
	}

	return db
}

func setup_endpoints(db *gorm.DB, enableLogging bool) *gin.Engine {
	var r *gin.Engine
	if enableLogging {
		gin.DisableConsoleColor()
		f, _ := os.Create("addressbook.log")
		gin.DefaultWriter = io.MultiWriter(f)
		r = gin.Default()
	} else {
		r = gin.New()
	}

	r.GET("/address", func(c *gin.Context) {
		var query struct {
			Sort   *string `form:"sort"`
			Search *string `form:"search"`
			Last   *string `form:"last"`
			Limit  *int    `form:"limit"`
		}

		if err := c.ShouldBindQuery(&query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		qdb := db
		// only allow 1 <= limit <= 100, default to 20
		if query.Limit != nil {
			if *query.Limit > 100 {
				qdb = qdb.Limit(100)
			} else if *query.Limit <= 0 {
				qdb = qdb.Limit(20)
			} else {
				qdb = qdb.Limit(*query.Limit)
			}
		} else {
			qdb = qdb.Limit(20)
		}

		// find matching prefixes
		// WARNING: wildcards in user input not escaped
		if query.Search != nil {
			qdb = qdb.Where("first_name LIKE ? || '%' OR last_name LIKE ? || '%'", *query.Search, *query.Search)
		}

		// handle sorting and pagination
		if query.Sort != nil && (*query.Sort == "first_name" || *query.Sort == "last_name") {
			// pagination assumes id as secondary sort key
			qdb = qdb.Order(*query.Sort).Order("id")

			// pagination
			if query.Last != nil {
				// the pagination key is on the form ID__NAME
				// describing the last result of the previous query
				prev_parts := strings.Split(*query.Last, "__")
				if len(prev_parts) < 2 {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid last key: %v", *query.Last)})
					return
				}

				prev_id, err := strconv.Atoi(prev_parts[0])
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				// on the off chance that the name contained a __
				prev_name := strings.Join(prev_parts[1:], "__")

				// cursor pagination. needs benchmarking
				// WARNING: possible sql injection if query.Sort not sanity checked
				qdb = qdb.Where(fmt.Sprintf("(id > ? AND %v = ?) OR  %v > ?", *query.Sort, *query.Sort), prev_id, prev_name, prev_name)
			}
		} else {
			qdb.Order("id")
			if query.Last != nil {
				id, err := strconv.Atoi(*query.Last)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				qdb = qdb.Where("id > ?", id)
			}
		}

		var addresses []Addresses
		result := qdb.Find(&addresses)
		if result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": result.Error.Error()})
		}

		// construct token for pagination - ID__NAME when sorting by name, ID otherwise
		var next_token string
		if len(addresses) > 0 {
			last_address := addresses[len(addresses)-1]
			if query.Sort != nil && *query.Sort == "first_name" {
				next_token = fmt.Sprintf("%v__%v", last_address.ID, last_address.FirstName)
			} else if query.Sort != nil && *query.Sort == "last_name" {
				next_token = fmt.Sprintf("%v__%v", last_address.ID, last_address.LastName)
			} else {
				next_token = fmt.Sprintf("%v", last_address.ID)
			}
		}

		response := gin.H{
			"success":   true,
			"addresses": addresses,
		}
		if len(next_token) > 0 {
			url := c.Request.URL
			url_query := url.Query()
			url_query.Set("last", next_token)
			url.RawQuery = url_query.Encode()
			response["next"] = url.String()
		} else {
			response["next"] = nil
		}

		c.JSON(http.StatusOK, response)
	})

	r.GET("/address/:id", func(c *gin.Context) {
		var address Addresses
		result := db.First(&address, c.Param("id"))
		if result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": result.Error.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"id":         address.ID,
			"first_name": address.FirstName,
			"last_name":  address.LastName,
			"phone":      address.Phone,
		})
	})

	r.DELETE("/address/:id", func(c *gin.Context) {
		result := db.Delete(&Addresses{}, c.Param("id"))
		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("no entry with id %v", c.Param("id"))})
		} else {
			c.JSON(http.StatusOK, gin.H{"success": true})
		}
	})

	r.POST("/address", func(c *gin.Context) {
		var json struct {
			FirstName string  `json:"first_name" binding:"required"`
			LastName  string  `json:"last_name" binding:"required"`
			Phone     *string `json:"phone"`
		}

		if err := c.ShouldBindJSON(&json); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		address := Addresses{FirstName: json.FirstName, LastName: json.LastName, Phone: json.Phone}
		result := db.Create(&address)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"success": true, "id": address.ID})
		}
	})

	return r
}

func main() {
	db := initialize_database("addressbook.db")
	r := setup_endpoints(db, true)
	r.Run()
}
