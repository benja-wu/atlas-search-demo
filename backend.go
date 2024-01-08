package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Global client instance
var clientInstance *mongo.Client

// Used for creating a singleton client instance
var clientInstanceError error
var mongoOnce sync.Once

// Database Config
const (
	CONNECTIONSTRING    = "YOUR_MONOGDB_CONN_STRING"
	DB                  = "YOUR_DATABASE"
	COLLECTION          = "items"
	CUSTOMER_COLLECTION = "customers"
)

// ReportItem is the web page post item
// for reporting user's click behavior
type ReportItem struct {
	Name       string `json:"name"  bson:"name" `
	DocumentId string `json:"documentId"  bson:"documentId"`
	Name2      string `json:"name2" bson:"name2"`

	ViewTime time.Time `json:"viewTime" bson:"viewTime"`
}

// Results are BSON array object, contains search items
type Results []bson.M

type SearchRsp struct {
	SearchResults       Results `json:"searchResults"`
	MoreLikeThisResults Results `json:"moreLikeThisResults"`
}

func main() {
	// Serve static files from the 'html' directory
	fs := http.FileServer(http.Dir("./html"))
	http.Handle("/", fs)

	// Handle /items for GET list requests
	http.HandleFunc("/items", itemsHandler)
	http.HandleFunc("/report-click", reportHandler)
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/search-p", personalizedSearchHandler)

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
	fmt.Println(results)
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
	fmt.Println("Received request with body:", string(body))
	var click ReportItem
	err = json.Unmarshal(body, &click)
	if err != nil {
		log.Fatal("Unmarshal the click report with error input body: %s, err: %v", string(body), err)
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

func pipelineC(query string, page int, views []bson.M) []bson.D {
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
	fmt.Printf("the pipeline is %v\n", p)
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
	return p
}

func personalizedSearchHandler(w http.ResponseWriter, r *http.Request) {
	pageNum := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageNum)
	if err != nil {
		page = 0
	}
	inputQuery := r.URL.Query().Get("query")

	searchItems := personalizedSearch(inputQuery, page)

	// Convert the data to JSON
	jsonData, err := json.Marshal(searchItems)
	if err != nil {
		http.Error(w, "Error converting data", http.StatusInternalServerError)
		return
	}

	// Set the Content-Type and write the JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
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
	inputQuery := r.URL.Query().Get("query")

	searchItems := search(inputQuery, page)

	// Convert the data to JSON
	jsonData, err := json.Marshal(searchItems)
	if err != nil {
		http.Error(w, "Error converting data", http.StatusInternalServerError)
		return
	}

	// Set the Content-Type and write the JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func personalizedSearch(query string, page int) SearchRsp {
	var rsp SearchRsp
	client, err := GetMongoClient()
	if err != nil {
		log.Fatal(err)
	}
	collection := client.Database(DB).Collection(COLLECTION)
	views := getRecentViewItems()
	p := pipelineC(query, page, views)

	cursor, err := collection.Aggregate(context.TODO(), p)
	if err != nil {
		fmt.Printf("search aggregate run failed, err %v", err)
		log.Fatal(err)
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		fmt.Printf("search cursor run failed, err %v", err)
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
	fmt.Printf("search pipeline is %v\n", p)

	cursor, err := collection.Aggregate(context.TODO(), p)
	if err != nil {
		fmt.Printf("search aggregate run failed, err %v", err)
		log.Fatal(err)
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		fmt.Printf("search cursor run failed, err %v", err)
		log.Fatal(err)
	}
	rsp.SearchResults = results
	rsp.MoreLikeThisResults = moreLikeThis()
	fmt.Printf("rsp is %v\n", rsp)
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
		fmt.Printf("get recent view item failed, err %v", err)
		log.Fatal(err)
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("the recent view items are %v\n", results)
	return results
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
	fmt.Printf("recently view itemid is %v", v)

	icollection := client.Database(DB).Collection(COLLECTION)
	var like bson.M
	if err := icollection.FindOne(context.TODO(), bson.M{"documentId": v["documentId"]}).Decode(&like); err != nil {
		fmt.Printf("get recent view item failed, err %v", err)
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
	//fmt.Printf("more like this pipeline %v", p)

	cursor, err := collection.Aggregate(context.TODO(), p)
	if err != nil {
		log.Fatal(err)
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("moreLieThis rsp is %v\n", results)
	return results
}
