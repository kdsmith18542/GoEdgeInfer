import requests
import time

# Base URL of the API
BASE_URL = "http://localhost:8080"

def test_rate_limit():
    """Test rate limiting by sending multiple requests."""
    print("Testing rate limiting...")
    print(f"Sending requests to {BASE_URL}")
    
    # First, check if the server is running
    try:
        response = requests.get(f"{BASE_URL}/health")
        print(f"Health check: {response.status_code} - {response.text}")
    except Exception as e:
        print(f"Error connecting to server: {e}")
        return
    
    # Test endpoint that should be rate limited (using /models which is in our rate-limited group)
    test_endpoint = f"{BASE_URL}/models"
    
    # Send 50 requests (more than our burst of 20)
    num_requests = 50
    success_count = 0
    rate_limited_count = 0
    error_count = 0
    
    print(f"\nSending {num_requests} requests in quick succession to {test_endpoint}...")
    print("First 20 requests should succeed (burst), then we should start getting rate limited")
    start_time = time.time()
    
    for i in range(num_requests):
        try:
            # Use a very small delay to send requests quickly
            time.sleep(0.01)
            
            response = requests.get(test_endpoint, timeout=2.0)
            if response.status_code == 200:
                success_count += 1
                if i < 20:  # First 20 should be in burst
                    print(f"Request {i+1}/{num_requests}: 200 OK (burst)")
                else:
                    print(f"Request {i+1}/{num_requests}: 200 OK (rate limit refill)")
            elif response.status_code == 429:  # Too Many Requests
                rate_limited_count += 1
                print(f"Request {i+1}/{num_requests}: 429 Rate Limited (expected after burst)")
            else:
                error_count += 1
                print(f"Request {i+1}/{num_requests}: {response.status_code} - {response.text}")
        except Exception as e:
            error_count += 1
            print(f"Request {i+1}/{num_requests}: Error - {e}")
    
    end_time = time.time()
    total_time = end_time - start_time
    
    print("\nTest Results:")
    print(f"Total requests: {num_requests}")
    print(f"Successful (200): {success_count}")
    print(f"Rate limited (429): {rate_limited_count}")
    print(f"Errors: {error_count}")
    print(f"Total time: {total_time:.2f} seconds")
    print(f"Requests per second: {num_requests/total_time:.2f}")

if __name__ == "__main__":
    test_rate_limit()
