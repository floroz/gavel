#!/bin/bash

BASE_URL="http://api.auction.local"

echo "Verifying Bid Service..."
# Sending an empty JSON to PlaceBid. Expecting a response (likely an error or empty success depending on validation).
# Content-Type application/json triggers ConnectRPC JSON protocol.
RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d '{}' "${BASE_URL}/bids.v1.BidService/PlaceBid")
echo "Bid Service Response: ${RESPONSE}"

if [[ "$RESPONSE" == *"code"* || "$RESPONSE" == *"{}"* ]]; then
    echo "✅ Bid Service is reachable"
else
    echo "❌ Bid Service unreachable or unexpected response"
fi

echo "Verifying User Stats Service..."
RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d '{}' "${BASE_URL}/userstats.v1.UserStatsService/GetUserStats")
echo "User Stats Service Response: ${RESPONSE}"

if [[ "$RESPONSE" == *"code"* || "$RESPONSE" == *"{}"* ]]; then
    echo "✅ User Stats Service is reachable"
else
    echo "❌ User Stats Service unreachable or unexpected response"
fi

