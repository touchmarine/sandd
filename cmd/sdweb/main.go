package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("GET /", home)
	log.Fatal(http.ListenAndServe("localhost:2472", nil))
}

func home(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
<title>Search & Data</title>
<style>
nav a {
    padding: 16px;
    border-radius: 8px;
    font-family: sans-serif;
    color: rgba(0, 0, 0, 0.8);
    text-decoration: none;
}
nav a:hover {
    background-color: rgba(0, 0, 0, 0.1);
}
</style>
</head>
<body>
<div style="display: flex; height: 100vh; justify-content: center; align-items: center;">
<nav style="display: flex; column-gap: 16px;">
<a href="https://cs.sd.test">Code Search</a>
<a href="https://jupyter.sd.test">Jupyter</a>
<a href="https://memos.sd.test">Memos</a>
</nav>
</div>
</body>
</html>
`))
}
