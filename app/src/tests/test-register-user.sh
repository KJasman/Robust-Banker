#!/bin/bash
# Semi-sloppy testing script with curl

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'
BLUE='\033[0;34m'

# Base endpoint 
BASE_URL="http://localhost:8000"

echo -e "${BLUE}Testing Authentication Service via API Gateway${NC}"
echo "==============================================="

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
# 1) Register a 'customer' user (fake distinction)
###################################################
echo -e "\n${BLUE}1. Testing User #1 (Customer) Registration...${NC}"
CUSTOMER_REGISTER_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/register" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "customer1",
    "password": "Customer@123",
    "name": "John Doe"
  }')
test_response "$CUSTOMER_REGISTER_RESPONSE" "User #1 (customer1) Registration"

###################################################
# 2) Register a 'company' user (fake distinction)
###################################################
echo -e "\n${BLUE}2. Testing User #2 (Company) Registration...${NC}"
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
echo -e "\n${BLUE}3. Testing Login (User #1)...${NC}"
CUSTOMER_LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "customer1",
    "password": "Customer@123"
  }')
test_response "$CUSTOMER_LOGIN_RESPONSE" "User #1 Login"

# Extract token for user #1
CUSTOMER_TOKEN=$(echo "$CUSTOMER_LOGIN_RESPONSE" | jq -r '.data.token')

###################################################
# 4) Login as user #2 (company1)
###################################################
echo -e "\n${BLUE}4. Testing Login (User #2)...${NC}"
COMPANY_LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "company1",
    "password": "Company@123"
  }')
test_response "$COMPANY_LOGIN_RESPONSE" "User #2 Login"

# Extract token for user #2
COMPANY_TOKEN=$(echo "$COMPANY_LOGIN_RESPONSE" | jq -r '.data.token')

###################################################
# 5) Register duplicate user #1
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
# 6) Register duplicate user #2
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

###################################################
# Display & Save Tokens
###################################################
echo -e "\n${BLUE}Generated Tokens:${NC}"
echo -e "User #1 (customer1) Token: ${GREEN}$CUSTOMER_TOKEN${NC}"
echo -e "User #2 (company1) Token: ${GREEN}$COMPANY_TOKEN${NC}"

echo -e "\n${BLUE}Test Suite Completed${NC}"

if [ -n "$CUSTOMER_TOKEN" ] && [ "$CUSTOMER_TOKEN" != "null" ]; then
  echo "$CUSTOMER_TOKEN" > customer_token.txt
  echo "Saved user #1 token to customer_token.txt"
fi

if [ -n "$COMPANY_TOKEN" ] && [ "$COMPANY_TOKEN" != "null" ]; then
  echo "$COMPANY_TOKEN" > company_token.txt
  echo "Saved user #2 token to company_token.txt"
fi