# Search & Data

1. Index the code directories: `go run github.com/google/codesearch/cmd/cindex $HOME/code`
2. Run the search web app: `go run cmd/csweb/web.go` (localhost:2473)
3. Add `127.0.0.1 memos.sd.test jupyter.sd.test cs.sd.test sd.test` to `/etc/hosts`
4. Run jupyter + memos + reverse proxy: `docker compose up`

Jupyter Notebooks are saved at ~/.sandd/jupyter/work. They are saved on host so they can be cindexed.

Memos are saved in ~/.sand/memos/memos_prod.db. They are not yet cindexed (as they are in sqlite).

## Why Discourse?

Discourse takes the place of a personal StackOverflow and GitHub Discussions.

### Difficulties

Discourse is built with Ruby on Rails, Postgres, Redis and requires complex bootstrapping. It seems like a nightmare to set up properly:
1. supported, standard install works only on Ubuntu as the pre-compose era launcher is written in bash ([launcher](https://github.com/discourse/discourse_docker/blob/eded2f8b5d8de2f678d7fadb4e998f90ee792d0c/launcher), [Docker-compose.yml to run discourse locally](https://meta.discourse.org/t/docker-compose-yml-to-run-discourse-locally/271826))
2. requires a domain and SMTP ([INSTALL-cloud.md](https://github.com/discourse/discourse/blob/acca39036b940988e15abf6f169e8a7f283e420b/docs/INSTALL-cloud.md))
3. issue regarding portable & bootstrap-less docker setup is open since 2015 without any visible progress ([Can Discourse ship frequent Docker images that do not need to be bootstrapped?](https://meta.discourse.org/t/can-discourse-ship-frequent-docker-images-that-do-not-need-to-be-bootstrapped/33205/177))
4. *hopefully launcher2 written in Go could be a solution ([discourse/discourse_docker#791](https://github.com/discourse/discourse_docker/pull/791))

[valleyhousingcoop/discourse-hosting](https://github.com/valleyhousingcoop/discourse-hosting) is a repository that provides a way to host Discourse on a server. On Jul 4, 2024, after almost two years, the maintaner posted this update:

> **UPDATE**: I found it too hard to maintain this setup especially accross discourse updates. I have since switched to the [standard discourse hosting setup](https://github.com/discourse/discourse/blob/main/docs/INSTALL-cloud.md), locally on a machine in my basement. That is much more stable and I have had no issues upgrading things. I would highly reccomend someone trying to approach to have a second thought about it, unless you have a very high amount of time investment available. I spent months on getting this working. I think long term for this kind of setup to work, it would have to be upstreamed into discourse and offically supported so that it stays working.

### Interim Knowledgebase - Memos

As Discourse requires more thought, Memos will serve as an interim knowledgebase.

Memos is lightweight note-taking service. That's it.
