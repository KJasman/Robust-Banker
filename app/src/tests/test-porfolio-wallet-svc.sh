#!/bin/bash

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'
BLUE='\033[0;34m'

BASE_URL="http://localhost:8000"

if [ -z "$CUSTOMER_TOKEN" ]; then
  if [ -f "customer_token.txt" ]; then
    CUSTOMER_TOKEN=$(cat customer_token.txt)
  fi
fi

if [ -z "$CUSTOMER_TOKEN" ]; then
  echo -e "${RED}Error: CUSTOMER_TOKEN not set.${NC}"
  echo "Please run auth test script or create 'customer_token.txt'."
  exit 1
fi

test_response() {
  local response="$1"
  local test_name="$2"

  if echo "$response" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ $test_name: Success${NC}"
  else
    echo -e "${RED}✗ $test_name: Failed${NC}"
    echo "Response: $response"
  fi
  echo "------------------------------"
}

echo -e "${BLUE}Testing Wallet/Transaction Endpoints via Gateway${NC}"
echo "================================================"

echo -e "${BLUE}1) POST /api/v1/transaction/addMoneyToWallet${NC}"
ADD_MONEY_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/transaction/addMoneyToWallet" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${CUSTOMER_TOKEN}" \
  -d '{"amount": 1000}')
test_response "$ADD_MONEY_RESPONSE" "Add Money to Wallet"

echo -e "${BLUE}2) GET /api/v1/transaction/getWalletBalance${NC}"
GET_BALANCE_RESPONSE=$(curl -s -X GET "${BASE_URL}/api/v1/transaction/getWalletBalance" \
  -H "Authorization: Bearer ${CUSTOMER_TOKEN}")
test_response "$GET_BALANCE_RESPONSE" "Get Wallet Balance"

BALANCE_VALUE=$(echo "$GET_BALANCE_RESPONSE" | jq -r '.data.balance')
echo "Current Balance: $BALANCE_VALUE"
echo "------------------------------"

echo -e "${BLUE}3) GET /api/v1/transaction/getWalletTransactions${NC}"
GET_TRANSACTIONS_RESPONSE=$(curl -s -X GET "${BASE_URL}/api/v1/transaction/getWalletTransactions" \
  -H "Authorization: Bearer ${CUSTOMER_TOKEN}")
test_response "$GET_TRANSACTIONS_RESPONSE" "Get Wallet Transactions"

FIRST_TX_ID=$(echo "$GET_TRANSACTIONS_RESPONSE" | jq -r '.data[0].wallet_tx_id')
echo "First Transaction ID: $FIRST_TX_ID"
echo "------------------------------"

echo -e "${BLUE}4) GET /api/v1/transaction/getStockPortfolio${NC}"
GET_PORTFOLIO_RESPONSE=$(curl -s -X GET "${BASE_URL}/api/v1/transaction/getStockPortfolio" \
  -H "Authorization: Bearer ${CUSTOMER_TOKEN}")
test_response "$GET_PORTFOLIO_RESPONSE" "Get Stock Portfolio"

echo -e "${BLUE}Test Suite Completed${NC}"