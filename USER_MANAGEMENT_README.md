# Multi-Tenant User Management System

## Overview

The Era Inventory API now includes a comprehensive multi-tenant user management system with hierarchical access control. The system supports:

- **Main Tenant (org_id = 1)**: Super admins who can manage users and data across ALL organizations
- **Client Tenants (org_id > 1)**: Regular clients who can ONLY manage users and data within their own organization

## 🚀 Key Features

### Multi-Tenant Architecture
- **Hierarchical Access Control**: Main tenant has global access, client tenants have org-scoped access
- **Row Level Security (RLS)**: Database-level enforcement of tenant isolation
- **Automatic Context**: User's organization context is automatically applied to all queries

### User Management
- **User Creation**: With tenant-aware organization assignment
- **Role-Based Access Control**: `viewer`, `project_admin`, `org_admin` roles
- **Profile Management**: Self-service profile updates and password changes
- **Secure Authentication**: JWT-based authentication with bcrypt password hashing

### Organization Management
- **Organization CRUD**: Create, read, update, delete organizations (main tenant only)
- **Organization Stats**: View user and data statistics per organization
- **Data Isolation**: Complete separation of data between organizations

## 🗄️ Database Schema

### Users Table
```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    org_id BIGINT NOT NULL REFERENCES organizations(id),
    roles TEXT[] NOT NULL DEFAULT '{"viewer"}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    last_login_at TIMESTAMPTZ
);
```

### Row Level Security Policies
All tables now have RLS policies that allow:
- **Main tenant (org_id = 1)**: Access to ALL data across organizations
- **Client tenants**: Access only to their own organization's data

## 🔐 Authentication & Authorization

### Available Roles
- **`viewer`**: Read-only access to organization data
- **`project_admin`**: Can manage projects and inventory within organization
- **`org_admin`**: Can manage users, sites, vendors, and all data within organization

### Access Control Matrix

| Action | Main Tenant | Client Tenant |
|--------|-------------|---------------|
| View all organizations | ✅ | ❌ |
| Create organizations | ✅ | ❌ |
| View all users | ✅ | ❌ (only own org) |
| Create users in any org | ✅ | ❌ (only own org) |
| Change user's org | ✅ | ❌ |
| View all inventory/sites/vendors | ✅ | ❌ (only own org) |
| Self-service profile/password | ✅ | ✅ |

## 🛠️ API Endpoints

### Public Endpoints (No Authentication)
```http
POST /auth/login              # User login
```

### User Management (org_admin required)
```http
GET    /users                 # List users (with optional org filter for main tenant)
POST   /users                 # Create user
GET    /users/{id}            # Get user details
PUT    /users/{id}            # Update user
DELETE /users/{id}            # Delete user
```

### Organization Management (main tenant only)
```http
GET    /organizations         # List all organizations
POST   /organizations         # Create organization
GET    /organizations/{id}    # Get organization details
PUT    /organizations/{id}    # Update organization
DELETE /organizations/{id}    # Delete organization
GET    /organizations/{id}/stats # Get organization statistics
```

### Self-Service Endpoints (all authenticated users)
```http
GET    /auth/profile          # Get current user profile
PUT    /auth/profile          # Update current user profile
PUT    /auth/change-password  # Change current user password
```

## 🔧 Setup Instructions

### 1. Run Database Migrations
```bash
# Apply the new migrations
psql -d your_database -f db/migrations/0007_users.sql
psql -d your_database -f db/migrations/0008_update_rls_multi_tenant.sql
```

### 2. Load Default Users
```bash
# Load default users (includes super admin and sample client admin)
psql -d your_database -f db/seeds/002_default_users.sql
```

### 3. Default Login Credentials
- **Super Admin**: `superadmin@maintenant.com` / `Password123!`
- **Client Admin**: `admin@clienta.com` / `Password123!`
- **Project Manager**: `manager@clienta.com` / `Password123!`
- **Viewer**: `viewer@clienta.com` / `Password123!`

⚠️ **Important**: Change default passwords in production!

## 📝 Usage Examples

### 1. Login as Super Admin
```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@maintenant.com",
    "password": "Password123!"
  }'
```

### 2. Create User in Specific Organization (Main Tenant)
```bash
curl -X POST http://localhost:8080/users \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newuser@clienta.com",
    "password": "SecurePassword123!",
    "first_name": "John",
    "last_name": "Doe",
    "org_id": 2,
    "roles": ["project_admin"]
  }'
```

### 3. Create Organization (Main Tenant Only)
```bash
curl -X POST http://localhost:8080/organizations \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "New Client Company"
  }'
```

### 4. List Users with Organization Filter
```bash
# Main tenant can filter by org_id
curl -X GET "http://localhost:8080/users?org_id=2" \
  -H "Authorization: Bearer YOUR_TOKEN"

# Client tenant automatically sees only their org users
curl -X GET "http://localhost:8080/users" \
  -H "Authorization: Bearer CLIENT_TOKEN"
```

## 🧪 Testing

Use the provided `test_user_management.http` file to test all endpoints and verify multi-tenant behavior:

1. Test login for both main tenant and client tenant users
2. Verify main tenant can see all organizations and users
3. Verify client tenant can only see their own organization's data
4. Test user creation with organization assignment
5. Test organization management (main tenant only)
6. Test self-service profile and password management

## 🔒 Security Features

### Password Security
- **bcrypt Hashing**: All passwords are hashed using bcrypt with default cost
- **Password Requirements**: Minimum 8 characters required
- **Secure Password Change**: Requires current password verification

### JWT Security
- **Secure Claims**: Includes user ID, organization ID, and roles
- **Token Validation**: Comprehensive validation including expiry, signature, and claims
- **Context Isolation**: Organization context automatically applied to all requests

### Database Security
- **Row Level Security**: Database-level enforcement of tenant isolation
- **Prepared Statements**: Protection against SQL injection
- **Input Validation**: Comprehensive validation of all user inputs

## 🚨 Important Notes

1. **Main Tenant Privileges**: The main tenant (org_id = 1) has global access to all data
2. **RLS Enforcement**: All data access is automatically filtered by organization
3. **Role Requirements**: User management requires `org_admin` role
4. **Organization Assignment**: Only main tenant can assign users to different organizations
5. **Last Admin Protection**: Cannot delete the last org_admin in an organization

## 🔄 Migration from Previous Version

If you're upgrading from a previous version:

1. All existing data will be assigned to org_id = 1 (main tenant) by default
2. You'll need to create proper organizations and reassign users as needed
3. The main tenant will initially have access to all existing data
4. Consider creating JWT tokens for existing users with appropriate roles

## 🎯 Success Criteria

✅ **Main tenant users can manage users across all organizations**  
✅ **Client tenant users can only manage users in their own organization**  
✅ **RLS policies automatically enforce data isolation**  
✅ **Main tenant can see all data, clients see only their data**  
✅ **Login works for users from any organization**  
✅ **Organization hierarchy is properly enforced**  

The multi-tenant user management system is now fully operational and ready for production use!
