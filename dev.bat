@echo off
nodemon --signal SIGKILL --ext go --exec "go run . || exit 1"
