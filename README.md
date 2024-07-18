# Search & Data

1. Index the code directories: `go run github.com/google/codesearch/cmd/cindex $HOME/code`
2. Run the search web app: `go run cmd/csweb/web.go` (localhost:2473)
3. Set host `cs.sd.test` and `jupyter.sd.test`
4. Run reverse proxy: `docker compose up`

Jupyter Notebooks are saved at ~/.sandd/jupyter/work. They are saved on host so they can be cindexed.
