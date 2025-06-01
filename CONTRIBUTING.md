# Contributing to aws-init

## Philosophy

- **Minimalism**: Every line must serve a purpose
- **Clarity**: Code should be readable by any Go programmer
- **Reliability**: Production-grade quality
- **No dependencies**: Keep the dependency tree minimal

## Development Process

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feature-name`
3. **Make** your changes following our standards
4. **Test** thoroughly: `make test`
5. **Submit** a pull request

## Code Standards

### Code Quality
```shell
go fmt ./...           # Format code
go vet ./...           # Check for issues  
golangci-lint run      # Lint code
go test -cover ./...   # Test with coverage
```

### Commit Messages
Use conventional commit format:
```
feat: add new secret parsing feature
fix: handle edge case in signal forwarding
docs: update README with new examples
test: add coverage for error paths
```

### Pull Request Requirements
- [ ] All tests pass
- [ ] Code is formatted and linted
- [ ] Coverage maintained or improved
- [ ] Documentation updated if needed
- [ ] Commit messages follow convention

## Testing

- Write table-driven tests
- Test error conditions
- No external dependencies in tests
- Aim for >40% coverage on new code

## What We Accept

- **Bug fixes**  
- **Performance improvements**  
- **Documentation improvements**  
- **Test coverage improvements**  
- **Security enhancements**

## What We Don't Accept

- **New dependencies** (without strong justification)  
- **Feature creep** (scope expansion)  
- **Complex abstractions**  
- **Breaking changes** (without major version bump)

## Review Process

1. **Automated checks** must pass (CI/CD)
2. **Maintainer review** required for merge
3. **Discussion** happens in PR comments
4. **Approval** needed before merge to main

## Release Process

- Tags follow semantic versioning: `v1.2.3`
- Releases are automated on tag push
- Breaking changes require major version bump

## Getting Help

- **Issues**: Use GitHub issues for bugs and feature requests
- **Questions**: Start a GitHub discussion
- **Security**: Email security issues privately

## License

By contributing, you agree your contributions will be licensed under the MIT License.