package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Global client instance
var clientInstance *mongo.Client
var log *logrus.Logger

// Used for creating a singleton client instance
var clientInstanceError error
var mongoOnce sync.Once

// Database Config
const (
	CONNECTIONSTRING            = "YOUR_MONOGDB_CONN_STRING"
	DB                          = "YOUR_DATABASE"
	COLLECTION                  = "items"
	CUSTOMER_COLLECTION         = "customers"
	SEARCH_REPORT_COLLECTION    = "searchs"
	MARKETING_CONFIG_COLLECTION = "marketing_config"
)

// ItemReport is the web page post item
// for reporting user's click behavior
type ItemReport struct {
	Name       string `json:"name"  bson:"name" `
	DocumentId string `json:"documentId"  bson:"documentId"`
	Name2      string `json:"name2" bson:"name2"`

	ViewTime time.Time `json:"viewTime" bson:"viewTime"`
}

// QueryReport is the user input search query, search time and
// total search this keyword counts, next page, previous page will
// calculate into one search operation
type QueryReport struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	User       string             `json:"name" bson:"name"`
	Query      string             `json:"query" bson:"query"`
	SearchTime time.Time          `json:"searchTime" bson:"searchTime"`
	Count      int                `json:"count" bson:"count"`
}

// PromotionConfig stores company product operator's promotion configurations
type PromotionConfig struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"`
	Status            string             `json:"status" bson:"status"`
	PromotionKeywords []string           `json:"promotionKeywords" bson:"promotionKeywords"`
	PromotionItemIDs  []string           `json:"PromotionItemIDs" bson:"PromotionItemIDs"`
	StartDate         time.Time          `json:"startDate" bson:"startDate"`
	EndDate           time.Time          `json:"endDate" bson:"endDate"`
}

// Results are BSON array object, contains search items
type Results []bson.M

type SearchRsp struct {
	SearchResults       Results `json:"searchResults"`
	MoreLikeThisResults Results `json:"moreLikeThisResults"`
}

func main() {
	file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logrus.Fatal("Failed to log to file, using default stderr")
	}
	defer file.Close()

	// Set up logrus
	log = logrus.New()
	log.Out = file

	// Serve static files from the 'html' directory
	fs := http.FileServer(http.Dir("./html"))
	http.Handle("/", fs)

	// Handle /items for GET list requests
	http.HandleFunc("/items", itemsHandler)
	http.HandleFunc("/report-click", reportHandler)
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/search-p", personalizedSearchHandler)
	http.HandleFunc("/search-m", marketingSearchHandler) // supporting company operator recommending items or keywords

	// Start the server
	http.ListenAndServe(":8080", nil)
}

// GetMongoClient is a function to create a singleton client instance.
func GetMongoClient() (*mongo.Client, error) {
	// Perform the client creation process only once.
	mongoOnce.Do(func() {
		clientOptions := options.Client().ApplyURI(CONNECTIONSTRING)
		client, err := mongo.Connect(context.TODO(), clientOptions)
		if err != nil {
			clientInstanceError = err
			return
		}
		// Check the connection
		err = client.Ping(context.TODO(), nil)
		if err != nil {
			clientInstanceError = err
			return
		}
		clientInstance = client
	})
	return clientInstance, clientInstanceError
}

func getItemList(skip int) Results {
	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}

	// Specify the collection
	collection := client.Database(DB).Collection(COLLECTION)

	// Perform an Aggregation
	//matchStage := bson.D{{"$match", bson.D{{"field", "value"}}}}                                       // Replace "field" and "value" with your actual parameters
	//groupStage := bson.D{{"$group", bson.D{{"_id", "$field"}, {"total", bson.D{{"$sum", "$field"}}}}}} // Replace as needed
	sortStage := bson.D{{"$sort", bson.D{{"documentId", -1}}}} // Replace "field" and "value" with your actual parameters
	limitStage := bson.D{{"$limit", 10}}
	projectStage := bson.D{
		{
			Key: "$project", Value: bson.D{
				{"_id", 0},
				{"name", 1},
				{"name2", 1},
				{"price", 1},
				{"imageUrl", 1},
				{"imageUrl2", 1},
				{"documentId", 1},
			},
		},
	}
	skipStage := bson.D{{"$skip", (skip - 1) * 10}}
	// Modify according to your needs
	pipe := mongo.Pipeline{sortStage, limitStage, projectStage}

	if skip > 1 {
		pipe = mongo.Pipeline{sortStage, skipStage, limitStage, projectStage}
	}

	cursor, err := collection.Aggregate(context.TODO(), pipe)
	if err != nil {
		log.Fatal(err)
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		log.Fatal(err)
	}
	log.Info(results)
	return results
}

func itemsHandler(w http.ResponseWriter, r *http.Request) {
	// Handle search queries
	pageNum := r.URL.Query().Get("page")
	// If a query exists, filter items
	page, err := strconv.Atoi(pageNum)
	if err != nil {
		page = 0
	}
	items := getItemList(page)

	// Convert the data to JSON
	jsonData, err := json.Marshal(items)
	if err != nil {
		http.Error(w, "Error converting data", http.StatusInternalServerError)
		return
	}

	// Set the Content-Type and write the JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

// reportHandler reports the user's click behavior in the list page
func reportHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// It's a good practice to close the body when you're done with it
	defer r.Body.Close()

	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}
	collection := client.Database(DB).Collection(CUSTOMER_COLLECTION)

	// Print the body to the server's console
	log.Info("Received request with body:", string(body))
	var click ItemReport
	err = json.Unmarshal(body, &click)
	if err != nil {
		log.WithFields(
			logrus.Fields{
				"body": body,
				"err":  err,
			},
		).Fatal("unmarshal the click report with error input body failed")
	}

	click.ViewTime = time.Now()

	// get the user doc from mongoDB
	var doc bson.M
	if err := collection.FindOne(context.TODO(), bson.M{"name": "benjamin"}).Decode(&doc); err != nil {
		log.Fatal(err)
	}

	clicks := doc["viewHistory"].(bson.A) // Change "clicks" to the actual field name of the array.
	clicks = append(clicks, click)

	// Step 3: Make sure the limitation (FIFO).
	if len(clicks) > 20 {
		clicks = clicks[len(clicks)-20:] // Keep only the 20 most recent elements.
	}

	update := bson.M{"$set": bson.M{"viewHistory": clicks}}
	if _, err := collection.UpdateOne(context.TODO(), bson.M{"name": "benjamin"}, update); err != nil {
		log.Fatal(err)
	}
}

// pipelineP M means marking promotion
func pipelineM(query string, page int, config *PromotionConfig) []bson.D {
	var searchStage bson.D
	searchStage = bson.D{
		{"$search", bson.D{
			{"index", "item_search2"},
			{"compound", bson.D{
				{"should", bson.A{
					bson.D{{"text", bson.D{{"query", query}, {"path", "name2"}, {"score", bson.D{{"boost", bson.D{{"value", 20}}}}}}}},
					bson.D{{"text", bson.D{{"query", query}, {"path", "name"}, {"score", bson.D{{"boost", bson.D{{"value", 15}}}}}}}},
				}},
				{"minimumShouldMatch", 1},
			}},
		}},
	}
	if config != nil {
		if len(config.PromotionKeywords) != 0 {

		}
		query += " OR ("
		for k, v := range config.PromotionKeywords {
			if k == len(config.PromotionKeywords)-1 {
				query += v
			} else {
				query += v + " OR "
			}
		}
		query += ")"
		log.WithFields(
			logrus.Fields{
				"formed query": query,
			},
		).Info("the enhanced query is")
		searchStage = bson.D{
			{"$search", bson.D{
				{"index", "item_search2"},
				{"compound", bson.D{
					{"should", bson.A{
						bson.D{{"queryString", bson.D{{"query", query}, {"defaultPath", "name2"}, {"score", bson.D{{"boost", bson.D{{"value", 20}}}}}}}},
						bson.D{{"queryString", bson.D{{"query", query}, {"defaultPath", "name"}, {"score", bson.D{{"boost", bson.D{{"value", 15}}}}}}}},
					}},
					{"minimumShouldMatch", 1},
				}},
			}},
		}
	}

	limitStage := bson.D{{"$limit", 10}}
	projectStage := bson.D{
		{
			Key: "$project", Value: bson.D{
				{"_id", 0},
				{"name", 1},
				{"name2", 1},
				{"price", 1},
				{"imageUrl", 1},
				{"imageUrl2", 1},
				{"documentId", 1},
			},
		},
	}
	skipStage := bson.D{{"$skip", (page - 1) * 10}}
	// Modify according to your needs
	p := mongo.Pipeline{searchStage, limitStage, projectStage}
	if page > 1 {
		p = mongo.Pipeline{searchStage, skipStage, limitStage, projectStage}
	}
	log.WithFields(
		logrus.Fields{
			"query":   query,
			"pipline": p,
		},
	).Info("generated pipline finished")
	return p
}

// pipelineP P means personalized
func pipelineP(query string, page int, views []bson.M) []bson.D {
	searchStage := bson.D{
		{"$search", bson.D{
			{"index", "item_search2"},
			{"compound", bson.D{
				{"should", bson.A{
					bson.D{{"text", bson.D{{"query", query}, {"path", "name2"}, {"score", bson.D{{"boost", bson.D{{"value", 20}}}}}}}},
					bson.D{{"text", bson.D{{"query", query}, {"path", "name"}, {"score", bson.D{{"boost", bson.D{{"value", 15}}}}}}}},
					bson.D{{"text", bson.D{{"query", query}, {"path", "discountTag"}}}},
					bson.D{{"moreLikeThis", bson.D{{"like", views}}}},
				}},
				{"minimumShouldMatch", 1},
			}},
		}},
	}
	limitStage := bson.D{{"$limit", 10}}
	projectStage := bson.D{
		{
			Key: "$project", Value: bson.D{
				{"_id", 0},
				{"name", 1},
				{"name2", 1},
				{"price", 1},
				{"imageUrl", 1},
				{"imageUrl2", 1},
				{"documentId", 1},
			},
		},
	}
	skipStage := bson.D{{"$skip", (page - 1) * 10}}
	// Modify according to your needs
	p := mongo.Pipeline{searchStage, limitStage, projectStage}
	if page > 1 {
		p = mongo.Pipeline{searchStage, skipStage, limitStage, projectStage}
	}
	log.WithFields(
		logrus.Fields{
			"query":   query,
			"pipline": p,
		},
	).Info("generated pipline finished")
	return p
}

func pipeline(query string, page int) []bson.D {
	searchStage := bson.D{
		{"$search", bson.D{
			{"index", "item_search2"},
			{"compound", bson.D{
				{"should", bson.A{
					bson.D{{"text", bson.D{{"query", query}, {"path", "name2"}, {"score", bson.D{{"boost", bson.D{{"value", 3}}}}}}}},
					bson.D{{"text", bson.D{{"query", query}, {"path", "name"}}}},
					bson.D{{"text", bson.D{{"query", query}, {"path", "discountTag"}}}},
				}},
				{"minimumShouldMatch", 1},
			}},
		}},
	}
	limitStage := bson.D{{"$limit", 10}}
	projectStage := bson.D{
		{
			Key: "$project", Value: bson.D{
				{"_id", 0},
				{"name", 1},
				{"name2", 1},
				{"price", 1},
				{"imageUrl", 1},
				{"imageUrl2", 1},
				{"documentId", 1},
			},
		},
	}
	skipStage := bson.D{{"$skip", (page - 1) * 10}}
	// Modify according to your needs
	p := mongo.Pipeline{searchStage, limitStage, projectStage}

	if page > 1 {
		p = mongo.Pipeline{searchStage, skipStage, limitStage, projectStage}
	}

	log.WithFields(
		logrus.Fields{
			"query":   query,
			"pipline": p,
		},
	).Info("generated pipline finished")
	return p
}

func personalizedSearchHandler(w http.ResponseWriter, r *http.Request) {
	pageNum := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageNum)
	if err != nil {
		page = 0
	}
	query := r.URL.Query().Get("query")

	searchItems := personalizedSearch(query, page)

	// Convert the data to JSON
	jsonData, err := json.Marshal(searchItems)
	if err != nil {
		http.Error(w, "Error converting data", http.StatusInternalServerError)
		return
	}
	go queryReport(query)

	// Set the Content-Type and write the JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func marketingSearchHandler(w http.ResponseWriter, r *http.Request) {
	pageNum := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageNum)
	if err != nil {
		page = 0
	}
	query := r.URL.Query().Get("query")

	searchItems := marktingSearch(query, page)

	// Convert the data to JSON
	jsonData, err := json.Marshal(searchItems)
	if err != nil {
		http.Error(w, "Error converting data", http.StatusInternalServerError)
		return
	}
	go queryReport(query)

	// Set the Content-Type and write the JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func queryReport(query string) {
	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}
	collection := client.Database(DB).Collection(SEARCH_REPORT_COLLECTION)

	var searchReport QueryReport
	filter := bson.M{"name": "benjamin", "query": query}
	if err = collection.FindOne(context.TODO(), filter).Decode(&searchReport); err != nil && err != mongo.ErrNoDocuments {
		log.WithFields(
			logrus.Fields{
				"query": query,
				"err":   err,
			},
		).Error("get user search query from mongoDB failed")
		return
	}

	log.WithFields(
		logrus.Fields{
			"query":  query,
			"result": searchReport,
			"err":    err,
		}).Info("search one query result ")
	if err == mongo.ErrNoDocuments {
		searchReport.Count = 1
		searchReport.Query = query
		searchReport.SearchTime = time.Now()
		searchReport.User = "benjamin"
	} else {
		searchReport.Count++
	}

	update := bson.M{
		"$set": searchReport,
	}
	// Set the Upsert option to true
	opts := options.Update().SetUpsert(true)

	if _, err := collection.UpdateOne(context.TODO(), filter, update, opts); err != nil {
		log.Error(err)
	}
}

// searchHandler accept the search request, search the match items
// and provided moreLikeThis recommendation.
func searchHandler(w http.ResponseWriter, r *http.Request) {
	pageNum := r.URL.Query().Get("page")
	// If a query exists, filter items
	page, err := strconv.Atoi(pageNum)
	if err != nil {
		page = 0
	}
	query := r.URL.Query().Get("query")

	searchItems := search(query, page)

	// Convert the data to JSON
	jsonData, err := json.Marshal(searchItems)
	if err != nil {
		http.Error(w, "Error converting data", http.StatusInternalServerError)
		return
	}

	go queryReport(query)
	// Set the Content-Type and write the JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

// personalizedSearch will merge the user-activity-based recommendation with
// user input keywords search result as response
func personalizedSearch(query string, page int) SearchRsp {
	var rsp SearchRsp
	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}
	collection := client.Database(DB).Collection(COLLECTION)
	views := getRecentViewItems()
	p := pipelineP(query, page, views)

	cursor, err := collection.Aggregate(context.TODO(), p)
	if err != nil {
		log.Fatal(err)
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		log.Fatal(err)
	}
	rsp.SearchResults = results
	return rsp
}

// marktingSearch will merge the commany operator configured promotion items with
// user input keywords search result as response
func marktingSearch(query string, page int) SearchRsp {
	var rsp SearchRsp
	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}
	collection := client.Database(DB).Collection(COLLECTION)
	config := getMarketingConfig()
	p := pipelineM(query, page, config)

	cursor, err := collection.Aggregate(context.TODO(), p)
	if err != nil {
		log.Fatal(err)
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		log.Fatal(err)
	}
	rsp.SearchResults = results
	return rsp
}

// search ask Atlas search for the text search
// pipeline: { "$search": { "index": "item_search2", "compound": { "should": [ { "text": { "query": "白", "path": "name2", "score": { "boost": { "value": 3 } } } }, { "text": { "query": "白", "path": "name" } }, { "text": { "query": "白", "path": "discountTag" } } ], "minimumShouldMatch": 1 } } }
func search(query string, page int) SearchRsp {
	var rsp SearchRsp
	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}
	collection := client.Database(DB).Collection(COLLECTION)

	p := pipeline(query, page)

	cursor, err := collection.Aggregate(context.TODO(), p)
	if err != nil {
		log.Fatal(err)
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		log.Fatal(err)
	}
	rsp.SearchResults = results
	rsp.MoreLikeThisResults = moreLikeThis()
	return rsp
}

func moreLikePipe(like bson.M) []bson.D {
	searchStage := bson.D{
		{"$search", bson.D{
			{"index", "item_search2"},
			{"moreLikeThis", bson.D{
				{"like", like},
			}},
		}},
	}
	limitStage := bson.D{{"$limit", 20}}
	projectStage := bson.D{
		{
			Key: "$project", Value: bson.D{
				{"_id", 0},
				{"name", 1},
				{"name2", 1},
				{"price", 1},
				{"imageUrl", 1},
				{"imageUrl2", 1},
				{"documentId", 1},
			},
		},
	}
	// Modify according to your needs
	p := mongo.Pipeline{searchStage, limitStage, projectStage}

	return p
}

// getRecentViewItems gets the user's view history, and get latest 5 items
// as response
func getRecentViewItems() []bson.M {
	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}
	collection := client.Database(DB).Collection(CUSTOMER_COLLECTION)

	var doc bson.M
	if err := collection.FindOne(context.TODO(), bson.M{"name": "benjamin"}).Decode(&doc); err != nil {
		log.Fatal(err)
	}
	views := doc["viewHistory"].(bson.A)
	var IDs []string
	for i := len(views) - 1; i > 0; i-- {
		IDs = append(IDs, views[i].(bson.M)["documentId"].(string))
		if len(IDs) == 5 {
			break
		}
	}

	// Create a projection
	projection := bson.M{
		"name":  1,
		"name2": 1,
	}

	// Set the projection in the find options
	findOptions := options.Find().SetProjection(projection)
	icollection := client.Database(DB).Collection(COLLECTION)
	cursor, err := icollection.Find(context.TODO(), bson.M{"documentId": bson.M{"$in": IDs}}, findOptions)

	if err != nil {
		log.Fatal(err)
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		log.Fatal(err)
	}
	return results
}

// getMarketingConfig gets the active promotion configuration from DB
func getMarketingConfig() *PromotionConfig {
	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}
	collection := client.Database(DB).Collection(MARKETING_CONFIG_COLLECTION)
	filter := bson.M{
		"status": "active",
		"startDate": bson.M{
			"$lte": time.Now(),
		},
		"endDate": bson.M{
			"$gte": time.Now(),
		},
	}

	// Fetch the document
	var p PromotionConfig
	err = collection.FindOne(context.TODO(), filter).Decode(&p)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.WithFields(
				logrus.Fields{
					"filter": filter,
				}).Info("No active promotion found")
			return nil
		} else {
			log.Fatal(err)
		}
	}
	return &p
}

func getRecentViewItem() bson.M {
	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}
	collection := client.Database(DB).Collection(CUSTOMER_COLLECTION)

	var doc bson.M
	if err := collection.FindOne(context.TODO(), bson.M{"name": "benjamin"}).Decode(&doc); err != nil {
		log.Fatal(err)
	}
	views := doc["viewHistory"].(bson.A)
	v := views[len(views)-1].(bson.M)
	log.WithFields(
		logrus.Fields{
			"recentViewItems": v,
		}).Info("get user recent view history")

	icollection := client.Database(DB).Collection(COLLECTION)
	var like bson.M
	if err := icollection.FindOne(context.TODO(), bson.M{"documentId": v["documentId"]}).Decode(&like); err != nil {
		log.Fatal(err)
	}
	return like
}

func moreLikeThis() Results {
	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database(DB).Collection(COLLECTION)
	like := getRecentViewItem()
	p := moreLikePipe(like)

	cursor, err := collection.Aggregate(context.TODO(), p)
	if err != nil {
		log.Fatal(err)
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		log.Fatal(err)
	}
	return results
}
