#!/bin/bash
# unorthodox semi-sloppy testing script with curl
# Define the base URL
BASE_URL="http://localhost:8080"

echo "Testing Authentication Service"
echo "----------------------------"

# Test 1: Register a new user
echo "1. Testing user registration..."
REGISTER_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/register" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "testuser",
    "password": "testpass123",
    "name": "Test User"
  }')
echo "Register Response: $REGISTER_RESPONSE"
echo "----------------------------"

# Test 2: Login with correct credentials
echo "2. Testing login with correct credentials..."
LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "testuser",
    "password": "testpass123"
  }')
echo "Login Response: $LOGIN_RESPONSE"
echo "----------------------------"

# Test 3: Login with incorrect password
echo "3. Testing login with incorrect password..."
FAILED_LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/login" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "testuser",
    "password": "wrongpassword"
  }')
echo "Failed Login Response: $FAILED_LOGIN_RESPONSE"
echo "----------------------------"

# Test 4: Register with existing username
echo "4. Testing registration with existing username..."
DUPLICATE_REGISTER_RESPONSE=$(curl -s -X POST "${BASE_URL}/authentication/register" \
  -H "Content-Type: application/json" \
  -d '{
    "user_name": "testuser",
    "password": "testpass123",
    "name": "Test User"
  }')
echo "Duplicate Register Response: $DUPLICATE_REGISTER_RESPONSE"