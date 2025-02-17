#!/bin/bash

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color
BLUE='\033[0;34m'

# Define the base URL for authentication and order services
AUTH_BASE_URL="http://localhost:8080"
ORDER_BASE_URL="http://localhost:8000/api/v1"

echo -e "${BLUE}Testing Authentication Service${NC}"
echo "=============================="

# Helper function to test response
test_response() {
    local response=$1
    local test_name=$2
    
    if echo "$response" | grep -q '"success":true'; then
        echo -e "${GREEN}✓ $test_name: Success${NC}"
    else
        echo -e "${RED}✗ $test_name: Failed${NC}"
        echo "Response: $response"
    fi
    echo "------------------------------"
}

# Test 1: Register a customer
echo -e "\n${BLUE}1. Testing Customer Registration...${NC}"
CUSTOMER_REGISTER_RESPONSE=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/register/customer" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "customer1",
    "password": "Customer@123",
    "name": "John Doe"
  }')
test_response "$CUSTOMER_REGISTER_RESPONSE" "Customer Registration"

# Test 2: Register a company
echo -e "\n${BLUE}2. Testing Company Registration...${NC}"
COMPANY_REGISTER_RESPONSE=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/register/company" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "company1",
    "password": "Company@123",
    "company_name": "Test Trading Corp"
  }')
test_response "$COMPANY_REGISTER_RESPONSE" "Company Registration"

# Test 3: Login as customer
echo -e "\n${BLUE}3. Testing Customer Login...${NC}"
CUSTOMER_LOGIN_RESPONSE=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "customer1",
    "password": "Customer@123"
  }')
test_response "$CUSTOMER_LOGIN_RESPONSE" "Customer Login"

# Extract customer token
CUSTOMER_TOKEN=$(echo $CUSTOMER_LOGIN_RESPONSE | jq -r '.data.token')

# Test 4: Login as company
echo -e "\n${BLUE}4. Testing Company Login...${NC}"
COMPANY_LOGIN_RESPONSE=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "company1",
    "password": "Company@123"
  }')
test_response "$COMPANY_LOGIN_RESPONSE" "Company Login"

# Extract company token
COMPANY_TOKEN=$(echo $COMPANY_LOGIN_RESPONSE | jq -r '.data.token')

# Test 5: Register duplicate customer
echo -e "\n${BLUE}5. Testing Duplicate Customer Registration...${NC}"
DUPLICATE_CUSTOMER_RESPONSE=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/register/customer" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "customer1",
    "password": "Different@123",
    "name": "Jane Doe"
  }')
if echo "$DUPLICATE_CUSTOMER_RESPONSE" | grep -q '"success":false'; then
    echo -e "${GREEN}✓ Duplicate Customer Prevention: Success${NC}"
else
    echo -e "${RED}✗ Duplicate Customer Prevention: Failed${NC}"
fi
echo "------------------------------"

# Test 6: Register duplicate company
echo -e "\n${BLUE}6. Testing Duplicate Company Registration...${NC}"
DUPLICATE_COMPANY_RESPONSE=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/register/company" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "company1",
    "password": "Different@123",
    "company_name": "Another Corp"
  }')
if echo "$DUPLICATE_COMPANY_RESPONSE" | grep -q '"success":false'; then
    echo -e "${GREEN}✓ Duplicate Company Prevention: Success${NC}"
else
    echo -e "${RED}✗ Duplicate Company Prevention: Failed${NC}"
fi
echo "------------------------------"

# Test 7: Login with incorrect password (Customer)
echo -e "\n${BLUE}7. Testing Login with Incorrect Password (Customer)...${NC}"
FAILED_CUSTOMER_LOGIN=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "customer1",
    "password": "WrongPass@123"
  }')
if echo "$FAILED_CUSTOMER_LOGIN" | grep -q '"success":false'; then
    echo -e "${GREEN}✓ Invalid Password Check: Success${NC}"
else
    echo -e "${RED}✗ Invalid Password Check: Failed${NC}"
fi
echo "------------------------------"

# Test 8: Login with incorrect password (Company)
echo -e "\n${BLUE}8. Testing Login with Incorrect Password (Company)...${NC}"
FAILED_COMPANY_LOGIN=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "company1",
    "password": "WrongPass@123"
  }')
if echo "$FAILED_COMPANY_LOGIN" | grep -q '"success":false'; then
    echo -e "${GREEN}✓ Invalid Password Check: Success${NC}"
else
    echo -e "${RED}✗ Invalid Password Check: Failed${NC}"
fi
echo "------------------------------"

# Display tokens for verification
echo -e "\n${BLUE}Generated Tokens:${NC}"
echo -e "Customer Token: ${GREEN}$CUSTOMER_TOKEN${NC}"
echo -e "Company Token: ${GREEN}$COMPANY_TOKEN${NC}"

# Test 9: Create a Stock (Company Side)
echo -e "\n${BLUE}11. Testing Create Stock...${NC}"
CREATE_STOCK_RESPONSE=$(curl -s -X POST "${ORDER_BASE_URL}/orders/createStock" \
  -H "Authorization: Bearer $COMPANY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "stock_name": "Apple"
  }')
test_response "$CREATE_STOCK_RESPONSE" "Create Stock"

echo -e "\n${BLUE}12. Testing Create Stock as Customer...${NC}"
CREATE_STOCK_FAILED=$(curl -s -X POST "${ORDER_BASE_URL}/orders/createStock" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" \
  -d '{
    "stock_name": "Amazon"
  }')
HTTP_STATUS_CODE=$CREATE_STOCK_FAILED
echo -e "${GREEN}HTTP Status Code:${NC} $HTTP_STATUS_CODE"

# Test 10: Place a Market Order
echo -e "\n${BLUE}9. Testing Place Market Buy Order...${NC}"
PLACE_ORDER_RESPONSE=$(curl -s -X POST "${ORDER_BASE_URL}/orders/placeStockOrder" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" \
  -d '{
    "stock_id": 2,
    "is_buy": true,
    "order_type": "MARKET",
    "quantity": 50
  }')
test_response "$PLACE_ORDER_RESPONSE" "Place Market Buy Order"

echo -e "\n${BLUE}10. Testing Place Market Sell Order...${NC}"
PLACE_ORDER_RESPONSE=$(curl -s -X POST "${ORDER_BASE_URL}/orders/placeStockOrder" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" \
  -d '{
    "stock_id": 2,
    "is_buy": false,
    "order_type": "MARKET",
    "quantity": 50
  }')
test_response "$PLACE_ORDER_RESPONSE" "Place Market Sell Order"

# Test 10: Place a Limit Order
echo -e "\n${BLUE}9. Testing Place Limit Buy Order...${NC}"
PLACE_ORDER_RESPONSE=$(curl -s -X POST "${ORDER_BASE_URL}/orders/placeStockOrder" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" \
  -d '{
    "stock_id": 2,
    "is_buy": true,
    "order_type": "LIMIT",
    "quantity": 550,
    "price": 120
  }')
test_response "$PLACE_ORDER_RESPONSE" "Place Limit Buy Order"

echo -e "\n${BLUE}10. Testing Place Limit Sell Order...${NC}"
PLACE_ORDER_RESPONSE=$(curl -s -X POST "${ORDER_BASE_URL}/orders/placeStockOrder" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" \
  -d '{
    "stock_id": 1,
    "is_buy": true,
    "order_type": "LIMIT",
    "quantity": 190,
    "price": 160
  }')
test_response "$PLACE_ORDER_RESPONSE" "Place Limit Sell Order"

# Test 11: Cancel a Stock Transaction
echo -e "\n${BLUE}10. Testing Cancel Stock Transaction...${NC}"
CANCEL_ORDER_RESPONSE=$(curl -s -X POST "${ORDER_BASE_URL}/orders/cancelStockTransaction" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" \
  -d '{
    "stock_tx_id": 1
  }')
test_response "$CANCEL_ORDER_RESPONSE" "Cancel Stock Transaction"


echo -e "\n${BLUE}Test Suite Completed${NC}"
