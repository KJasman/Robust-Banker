#!/bin/bash
# Semi-sloppy testing script with curl

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'
BLUE='\033[0;34m'

# Base endpoint
BASE_URL="http://localhost:8000"

echo -e "${BLUE}Testing Auth + Wallet + Order Services via API Gateway (No userType)${NC}"
echo "==================================================================="

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

###################################################
# 1) Register first user (#1)
###################################################
echo -e "\n${BLUE}1. Registering User #1 (customer1)...${NC}"
CUSTOMER_REGISTER_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/register" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "customer1",
    "password": "Customer@123",
    "name": "John Doe"
  }')
test_response "$CUSTOMER_REGISTER_RESPONSE" "User #1 (customer1) Registration"

###################################################
# 2) Register second user (#2)
###################################################
echo -e "\n${BLUE}2. Registering User #2 (company1)...${NC}"
COMPANY_REGISTER_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/register" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "company1",
    "password": "Company@123",
    "name": "Test Trading Corp"
  }')
test_response "$COMPANY_REGISTER_RESPONSE" "User #2 (company1) Registration"

###################################################
# 3) Login as user #1 (customer1)
###################################################
echo -e "\n${BLUE}3. Login (User #1: customer1)...${NC}"
CUSTOMER_LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "customer1",
    "password": "Customer@123"
  }')
test_response "$CUSTOMER_LOGIN_RESPONSE" "User #1 Login"

CUSTOMER_TOKEN=$(echo "$CUSTOMER_LOGIN_RESPONSE" | jq -r '.data.token')

###################################################
# 4) Login as user #2 (company1)
###################################################
echo -e "\n${BLUE}4. Login (User #2: company1)...${NC}"
COMPANY_LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "company1",
    "password": "Company@123"
  }')
test_response "$COMPANY_LOGIN_RESPONSE" "User #2 Login"

COMPANY_TOKEN=$(echo "$COMPANY_LOGIN_RESPONSE" | jq -r '.data.token')

###################################################
# 5) Test Duplicate Registration (User #1)
###################################################
echo -e "\n${BLUE}5. Testing Duplicate Registration (User #1)...${NC}"
DUPLICATE_CUSTOMER_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/register" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "customer1",
    "password": "Different@123",
    "name": "Jane Doe"
  }')
if echo "$DUPLICATE_CUSTOMER_RESPONSE" | grep -q '"success":false'; then
    echo -e "${GREEN}✓ Duplicate (customer1) Prevention: Success${NC}"
else
    echo -e "${RED}✗ Duplicate (customer1) Prevention: Failed${NC}"
    echo "Response: $DUPLICATE_CUSTOMER_RESPONSE"
fi
echo "------------------------------"

###################################################
# 6) Test Duplicate Registration (User #2)
###################################################
echo -e "\n${BLUE}6. Testing Duplicate Registration (User #2)...${NC}"
DUPLICATE_COMPANY_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/register" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "company1",
    "password": "Different@123",
    "name": "Another Corp"
  }')
if echo "$DUPLICATE_COMPANY_RESPONSE" | grep -q '"success":false'; then
    echo -e "${GREEN}✓ Duplicate (company1) Prevention: Success${NC}"
else
    echo -e "${RED}✗ Duplicate (company1) Prevention: Failed${NC}"
    echo "Response: $DUPLICATE_COMPANY_RESPONSE"
fi
echo "------------------------------"

###################################################
# 7) Login with incorrect password (User #1)
###################################################
echo -e "\n${BLUE}7. Testing Login with Incorrect Password (User #1)...${NC}"
FAILED_CUSTOMER_LOGIN=$(curl -s -X POST "${BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "customer1",
    "password": "WrongPass@123"
  }')
if echo "$FAILED_CUSTOMER_LOGIN" | grep -q '"success":false'; then
    echo -e "${GREEN}✓ Invalid Password Check (customer1): Success${NC}"
else
    echo -e "${RED}✗ Invalid Password Check (customer1): Failed${NC}"
    echo "Response: $FAILED_CUSTOMER_LOGIN"
fi
echo "------------------------------"

###################################################
# 8) Login with incorrect password (User #2)
###################################################
echo -e "\n${BLUE}8. Testing Login with Incorrect Password (User #2)...${NC}"
FAILED_COMPANY_LOGIN=$(curl -s -X POST "${BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "company1",
    "password": "WrongPass@123"
  }')
if echo "$FAILED_COMPANY_LOGIN" | grep -q '"success":false'; then
    echo -e "${GREEN}✓ Invalid Password Check (company1): Success${NC}"
else
    echo -e "${RED}✗ Invalid Password Check (company1): Failed${NC}"
    echo "Response: $FAILED_COMPANY_LOGIN"
fi
echo "------------------------------"

echo -e "\n${BLUE}Generated Tokens:${NC}"
echo -e "User #1 (customer1) Token: ${GREEN}$CUSTOMER_TOKEN${NC}"
echo -e "User #2 (company1) Token: ${GREEN}$COMPANY_TOKEN${NC}"

###################################################
# 9) createStock (user #2)
###################################################
echo -e "\n${BLUE}9. createStock as User #2 (company1)...${NC}"
CREATE_STOCK_RESPONSE=$(curl -s -X POST "${BASE_URL}/setup/createStock" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $COMPANY_TOKEN" \
  -d '{
    "stock_name": "TestStockInc"
  }')
test_response "$CREATE_STOCK_RESPONSE" "createStock"

# Extract the new stock_id
NEW_STOCK_ID=$(echo "$CREATE_STOCK_RESPONSE" | jq -r '.data.stock_id')
echo "NEW_STOCK_ID: $NEW_STOCK_ID"

###################################################
# 10) addStockToUser (user #2)
#     e.g. add 100 shares to user #1 (just an example)
###################################################
echo -e "\n${BLUE}10. addStockToUser as User #2 (company1)...${NC}"
ADD_STOCK_RESPONSE=$(curl -s -X POST "${BASE_URL}/setup/addStockToUser" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $COMPANY_TOKEN" \
  -d "{
    \"stock_id\": $NEW_STOCK_ID,
    \"quantity\": 100
  }")
test_response "$ADD_STOCK_RESPONSE" "addStockToUser"

###################################################
# 11) addMoneyToWallet (user #1 adds money)
###################################################
echo -e "\n${BLUE}11. addMoneyToWallet as User #1...${NC}"
ADD_MONEY_RESPONSE=$(curl -s -X POST "${BASE_URL}/transaction/addMoneyToWallet" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" \
  -d '{
    "amount": 5000
  }')
test_response "$ADD_MONEY_RESPONSE" "addMoneyToWallet (5000)"

###################################################
# 12) placeStockOrder (MARKET BUY) as user #1
###################################################
echo -e "\n${BLUE}12. placeStockOrder (MARKET BUY) as User #1...${NC}"
PLACE_ORDER_RESPONSE=$(curl -s -X POST "${BASE_URL}/engine/placeStockOrder" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" \
  -d "{
    \"stock_id\": $NEW_STOCK_ID,
    \"is_buy\": true,
    \"order_type\": \"MARKET\",
    \"quantity\": 10
  }")
test_response "$PLACE_ORDER_RESPONSE" "placeStockOrder (MARKET BUY)"

# Extract the stock_tx_id for possible cancellation
STOCK_TX_ID=$(echo "$PLACE_ORDER_RESPONSE" | jq -r '.data.stock_tx_id')
echo "STOCK_TX_ID: $STOCK_TX_ID"

###################################################
# 13) cancelStockTransaction (optional, user #1)
###################################################
echo -e "\n${BLUE}13. cancelStockTransaction as User #1 (optional)...${NC}"
CANCEL_RESPONSE=$(curl -s -X POST "${BASE_URL}/engine/cancelStockTransaction" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" \
  -d "{
    \"stock_tx_id\": \"$STOCK_TX_ID\"
  }")
test_response "$CANCEL_RESPONSE" "cancelStockTransaction"

###################################################
# 14) getWalletBalance (should reflect added money)
###################################################
echo -e "\n${BLUE}14. getWalletBalance (User #1)...${NC}"
GET_BALANCE_RESPONSE=$(curl -s -X GET "${BASE_URL}/transaction/getWalletBalance" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN")
test_response "$GET_BALANCE_RESPONSE" "getWalletBalance"

echo -e "Response: $GET_BALANCE_RESPONSE"

###################################################
# 15) getStockPortfolio (User #1)
###################################################
echo -e "\n${BLUE}15. getStockPortfolio (User #1)...${NC}"
GET_PORTFOLIO_RESPONSE=$(curl -s -X GET "${BASE_URL}/transaction/getStockPortfolio" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN")
test_response "$GET_PORTFOLIO_RESPONSE" "getStockPortfolio"
echo -e "Response: $GET_PORTFOLIO_RESPONSE"

echo -e "\n${BLUE}Test Suite Completed.${NC}"