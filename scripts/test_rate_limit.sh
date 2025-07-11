#!/bin/bash

# Test rate limiting by making multiple requests to the API
for i in {1..110}; do
    echo "Request $i/110"
    curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" http://localhost:8080/health
    # Sleep for a short time to avoid overwhelming the server
    sleep 0.1
done
