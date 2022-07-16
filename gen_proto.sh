#!/bin/sh
protoc -I internal/ --go_out internal/request/ internal/request/messages.proto