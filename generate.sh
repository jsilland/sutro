#!/bin/bash

swagger flatten https://developers.strava.com/swagger/swagger.json > swagger.json

# Make changes…

swagger generate client -f swagger.json -t . --template-dir=go-swagger-cli/templates --allow-template-override -C go-swagger-cli/config.yml