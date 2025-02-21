#!/bin/bash

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color
BLUE='\033[0;34m'

# Define the base URL for authentication and order services
AUTH_BASE_URL="http://localhost:8000"

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
echo -e "\n${BLUE}1. Testing Registration...${NC}"
CUSTOMER_REGISTER_RESPONSE=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/register" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name":"VanguardETF", 
    "password":"Vang@123", 
    "name":"Vanguard Corp."
  }')
test_response "$CUSTOMER_REGISTER_RESPONSE" "Registration"

echo -e "\n${BLUE}2. Testing Failed Registration...${NC}"
CUSTOMER_REGISTER_RESPONSE=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/register" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name":"VanguardETF", 
    "password":"Vang@12345", 
    "name":"Vanguard Ltd."
  }')
test_response "$CUSTOMER_REGISTER_RESPONSE" "Registration"

echo -e "\n${BLUE}3. Testing Login...${NC}"
CUSTOMER_REGISTER_RESPONSE=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name":"VanguardETF", 
    "password":"Vang@123"
  }')
test_response "$CUSTOMER_REGISTER_RESPONSE" "Login"

echo -e "\n${BLUE}3. Testing Failed Login...${NC}"
CUSTOMER_REGISTER_RESPONSE=$(curl -s -X POST "${AUTH_BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name":"VanguardETF", 
    "password":"Vang@1234566"
  }')
test_response "$CUSTOMER_REGISTER_RESPONSE" "Login"