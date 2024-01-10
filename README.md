# atlas-search-demo

## Background
1. MongoDB Atlas Search is the is a full-text search solution that offers a seamless and scalable experience for building relevance-based features.
2. Atlas search uses lucene engine as the reversed index search engine as ES/Solr.
3. This demo aims at create a demo with eShop search items scenario. It will return personalization of search results with recently viewed behaviors. It also supports comnay operator configure some promotion keywords for special sale events.  

## Collections 
* There are two main collections in this demo, customers and items. The `customers` collection stores website visitor's recent behavior and user tags. The `items` collection stores eShop item details for display and search.  

* The `marketing_config` collection stores company operator's configuration for special sale promotion search 
### Customers
Demo document:
```json
{
   "_id":{
      "$oid":"659bcd9024042cf68829b175"
   },
   "name":"benjamin",
   "register_date":{
      "$date":{
         "$numberLong":"1550293200000"
      }
   },
   "tags":[
      "romantic",
      "gold",
      "cartoon",
      "diy"
   ],
   "viewHistory":[
      {
         "name2":"物品中文名稱",
         "viewTime":{
            "$date":{
               "$numberLong":"1704639571425"
            }
         },
         "name":"item English name",
         "documentId":"xxxx-yyyy-zzzz"
      }
   ]
}
```

### Items 
Demo document:
```json
{
   "_id":{
      "$oid":"659bce3524042cf68829b176"
   },
   "documentId":"xxxx-yyyy-zzzz",
   "name":"item English name",
   "name2":"物品中文名稱",
   "originalPrice":{
      "$numberDouble":"5295.0"
   },
   "price":{
      "$numberDouble":"5295.0"
   },
   "ratio":{
      "$numberDouble":"1.0"
   },
   "imageUrl":"https://domain/item_pic.jpg",
   "imageUrl2":"https://domain/item_pic_big.jpg",

}
```


### Marketing_config 
Demo document:

```json
{
  "_id": { "$oid": "659e50986306c4fae5734903" },
  "status": "active",
  "promotionKeywords": ["手鐲", "薄荷"],
  "startDate": {
    "$date": { "$numberLong": "1704794839250" }
  },
  "endDate": {
    "$date": { "$numberLong": "1704967719250" }
  },
  "promotionItemIDs": [
    "94425B-24KG-00",
    "94445E-24KG-00"
  ]
}
```


## Search index 
Create the search index in Atlas webpage with the provided JSON configuration below

```json
{
  "mappings": {
    "dynamic": false,
    "fields": {
      "discountTag": {
        "multi": {
          "chinese": {
            "analyzer": "lucene.chinese",
            "searchAnalyzer": "lucene.chinese",
            "type": "string"
          },
          "english": {
            "analyzer": "lucene.english",
            "searchAnalyzer": "lucene.english",
            "type": "string"
          },
          "keyword": {
            "analyzer": "lucene.keyword",
            "searchAnalyzer": "lucene.keyword",
            "type": "string"
          }
        },
        "type": "string"
      },
      "documentId": {
        "analyzer": "lucene.standard",
        "type": "string"
      },
      "name": {
        "multi": {
          "chinese": {
            "analyzer": "lucene.chinese",
            "searchAnalyzer": "lucene.chinese",
            "type": "string"
          },
          "english": {
            "analyzer": "lucene.english",
            "searchAnalyzer": "lucene.english",
            "type": "string"
          },
          "keyword": {
            "analyzer": "lucene.keyword",
            "searchAnalyzer": "lucene.keyword",
            "type": "string"
          }
        },
        "type": "string"
      },
      "name2": {
        "multi": {
          "chinese": {
            "analyzer": "lucene.chinese",
            "searchAnalyzer": "lucene.chinese",
            "type": "string"
          },
          "english": {
            "analyzer": "lucene.english",
            "searchAnalyzer": "lucene.english",
            "type": "string"
          },
          "keyword": {
            "analyzer": "lucene.keyword",
            "searchAnalyzer": "lucene.keyword",
            "type": "string"
          }
        },
        "type": "string"
      },
      "originalPrice": {
        "type": "number"
      },
      "price": {
        "type": "number"
      },
      "productTag": {
        "analyzer": "lucene.standard",
        "type": "string"
      },
      "ratio": {
        "type": "number"
      }
    }
  }
}
```

## Procedure 
### Prerequest 
1. Prepare your MongoDB instance with demo colleciotns. 
2. Replace the `CONNECTION STRING`, `DB` fields in `backend.go` code.
3. Create the search index according to the provided configuration. 

### APIs
1. http://localhost:8080/ get the item lists from DB
2. http://localhost:8080/search search the item with user based recommendation  
3. http://localshot:8080/search-p search the item with user based recommendation with one merged result 
4. http://localshot:8080/search-m search the item with pre-configured promotion keywords with one merged result 

### Start backend server
* Use `go run backend.go` command to run the backend server 

### Search with webpage
1. Visit `http://localhost:8080/` to show the whole item lists.
2. Click the item picture will open a new tab and trigger one visiting behavior reporting to backend. So we can use the latest visit history to do the search recommendation.
3. Visit `http://localhost:8080/search.html` to search items. The web page contains two section, `search result` and `you may like`. 
4. Visit `http://localhost:8080/personalized_search.html` to search items. This web page will combine the user input query search result and recommendations based on user's recent visiting history together. 


## TO-DO
1. Add `synonyms` and other Atlas search features into this demo 
