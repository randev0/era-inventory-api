# Response Envelope Structure

All list endpoints now return responses wrapped in a standardized envelope format instead of raw JSON arrays.

## Response Format

```json
{
  "data": [...],
  "page": {
    "limit": <limit>,
    "offset": <offset>,
    "total": <total count of rows matching filter>
  }
}
```

## Updated Endpoints

The following endpoints now use the response envelope:

- `GET /items` - List inventory items
- `GET /sites` - List sites
- `GET /vendors` - List vendors  
- `GET /projects` - List projects

## Implementation Details

### 1. Response Envelope Structure

```go
type listResponse struct {
    Data []interface{} `json:"data"`
    Page pageInfo      `json:"page"`
}

type pageInfo struct {
    Limit  int `json:"limit"`
    Offset int `json:"offset"`
    Total  int `json:"total"`
}
```

### 2. Total Count Implementation

Each list endpoint now uses `COUNT(*) OVER()` in the SQL query to efficiently get the total count of rows matching the filter criteria:

```sql
SELECT id, name, ..., COUNT(*) OVER() as total_count
FROM table_name
WHERE conditions
ORDER BY ...
LIMIT ? OFFSET ?
```

### 3. Helper Function

A centralized `sendListResponse()` function ensures consistent envelope formatting across all endpoints:

```go
func sendListResponse(w http.ResponseWriter, data []interface{}, total int, params listParams)
```

## Example Responses

### Before (Raw Array)
```json
[
  {
    "id": 1,
    "name": "Access Switch",
    "asset_tag": "SW-001"
  },
  {
    "id": 2, 
    "name": "Core Router",
    "asset_tag": "RT-001"
  }
]
```

### After (With Envelope)
```json
{
  "data": [
    {
      "id": 1,
      "name": "Access Switch", 
      "asset_tag": "SW-001"
    },
    {
      "id": 2,
      "name": "Core Router",
      "asset_tag": "RT-001"
    }
  ],
  "page": {
    "limit": 50,
    "offset": 0,
    "total": 2
  }
}
```

## Benefits

1. **Consistent API Structure** - All list endpoints follow the same response format
2. **Pagination Metadata** - Clients can easily implement pagination controls
3. **Total Count** - Efficiently provides total matching records without additional queries
4. **Future Extensibility** - Envelope structure allows adding metadata without breaking changes

## Testing

You can test the new envelope structure using the existing HTTP requests in `request/api.http`:

```bash
# Start the services
docker-compose up -d

# Test list endpoints
curl http://localhost:8080/items
curl http://localhost:8080/sites  
curl http://localhost:8080/vendors
curl http://localhost:8080/projects
```

Each will now return the wrapped response format with pagination information.
