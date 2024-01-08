# atlas-search-demo

## Background
1. MongoDB Atlas Search is the is a full-text search solution that offers a seamless and scalable experience for building relevance-based features.
2. Atlas search uses lucene engine as the reversed index search engine as ES/Solr.
3. This demo aims at create a demo with eShop search items scenario. It will return personalization of search results with recently viewed behaviors. 

## Data schema
There are two collections in this demo, customers and items. The `customers` collection stores website visitor's recent behavior and user tags. The `items` collection stores eShop item details for display and search.  

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

## Procedure 
### Prerequest 
1. Prepare your MongoDB instance with demo colleciotns. 
2. Replace the `CONNECTION STRING`, `DB` fields in `backend.go` code.

### Start backend server
3. Use `go run backend.go` command to run the backend server 

### Search with webpage
1. Visit `http://localhost:8080/` to show the whole item lists.
2. Click the item picture will open a new tab and trigger one visiting behavior reporting to backend. So we can use the latest visit history to do the search recommendation.
3. Visit `http://localhost:8080/search.html` to search items. The web page contains two section, `search result` and `you may like`. 
4. Visit `http://localhost:8080/personalized_search.html` to search items. This web page will combine the user input query search result and recommendations based on user's recent visiting history together. 


## TO-DO
1. Add `synonyms` and other Atlas search features into this demo 
