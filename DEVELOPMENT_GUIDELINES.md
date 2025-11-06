# Vibe Coding Development Guidelines

## Core Principles

### 1. Minimalist yet Extensible Development
- Write code that is concise and focused on the essential functionality
- Design systems that can be easily extended without major refactoring
- Avoid over-engineering; implement only what's needed for the current requirements
- Use modular architecture to allow for future enhancements

### 2. DRY (Don't Repeat Yourself)
- Eliminate code duplication through abstraction and reuse
- Create reusable functions, classes, and modules
- Use inheritance, composition, and design patterns appropriately
- Regularly refactor to remove duplicated logic

### 3. Prefer Popular Open Source Libraries and Tools
- Research and utilize established open source solutions before building custom implementations
- Prioritize libraries with active maintenance, good documentation, and strong community support
- Evaluate alternatives based on:
  - Popularity and adoption rate
  - Maintenance status and update frequency
  - Compatibility with project requirements
  - License compatibility

### 5. Security and Sensitive Data Protection
- **Never commit sensitive information** to version control:
  - Database credentials (usernames, passwords, connection strings)
  - API keys and tokens
  - Private keys and certificates
  - Personal identifiable information (PII)
  - Internal server IPs, hostnames, or ports
- **Use environment variables** for sensitive configuration:
  - Database connection details
  - API keys and secrets
  - Service endpoints and ports
- **Implement proper .gitignore rules**:
  - Configuration files containing sensitive data
  - Local development files
  - Generated artifacts with sensitive information
- **Use configuration templates**:
  - Provide `config.example.yaml` with placeholder values
  - Document required environment variables
  - Include setup instructions for new developers
- **Regular security audits**:
  - Review git history for accidentally committed secrets
  - Use tools like `git-secrets` or `truffleHog` for automated scanning
  - Rotate credentials regularly
- **Access control and permissions**:
  - Use read-only database connections for analysis
  - Implement proper authentication for APIs
  - Limit data sampling to prevent information leakage

## Implementation Guidelines

### Security Implementation
- **Input validation and sanitization**:
  - Validate all user inputs
  - Sanitize database queries to prevent SQL injection
  - Use parameterized queries
- **Error handling**:
  - Don't expose sensitive information in error messages
  - Log security events appropriately
  - Implement proper exception handling
- **Data protection**:
  - Encrypt sensitive data at rest and in transit
  - Implement proper session management
  - Use HTTPS for all external communications

### Code Quality
- Follow Go best practices and conventions
- Use meaningful variable and function names
- Add comments for complex logic
- Maintain consistent code formatting

### Version Control
- Use descriptive commit messages
- Create feature branches for new developments
- Review code changes through pull requests
- Tag releases appropriately

### Project Structure
- Organize code into logical packages
- Separate concerns (business logic, data access, presentation)
- Keep configuration external and environment-specific

## Tools and Libraries Recommendations

### Development Tools
- Use popular Go IDEs (GoLand, VS Code with Go extensions)
- Implement linting with `golangci-lint`
- Use dependency management with Go modules

### Testing Frameworks
- Standard `testing` package for unit tests
- `testify` for enhanced assertions
- `httptest` for HTTP handler testing

### Popular Libraries
- Web frameworks: Gin, Echo
- Database: GORM, sqlx
- Configuration: Viper
- Logging: Zap, Logrus

## Continuous Improvement
- Regularly review and update these guidelines
- Incorporate lessons learned from project development
- Stay updated with Go ecosystem advancements
- Encourage knowledge sharing within the team