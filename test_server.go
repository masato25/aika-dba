package main

import (
"fmt"
"net/http"
"encoding/json"
)

func main() {
http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
if r.Method != "POST" {
http.Error(w, "Method not allowed", 405)
return
}

var req struct {
PartialQuery string `json:"partial_query"`
}

if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, err.Error(), 400)
return
}

response := map[string]interface{}{
"success": true,
"suggestions": []string{
"商品銷售統計",
"商品庫存查詢", 
"商品價格分析",
},
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(response)
})

fmt.Println("Test server running on :8080")
http.ListenAndServe(":8080", nil)
}
