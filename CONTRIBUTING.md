# Contributing to Politic Core

Thank you for your interest in contributing to Politic Core! This document provides guidelines for contributing.

## Code of Conduct

Be respectful, inclusive, and constructive. We're building tools for democratic participation.

## How to Contribute

### Reporting Issues

- Check existing issues before creating a new one
- Include reproduction steps, expected vs actual behavior
- For security issues, email tsksandeep11@gmail.com with subject "SECURITY"

### Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Run tests (`go test ./...`)
5. Commit with clear messages
6. Push and create a PR

### What We're Looking For

**High Priority:**
- Booth name corrections (especially regional language variations)
- Geographic data corrections (coordinates, boundaries)
- Bug fixes with test cases

**Welcome:**
- Performance improvements
- Test coverage improvements
- Documentation improvements
- Privacy algorithm enhancements

**Discuss First:**
- New packages or major features
- API changes to proto definitions
- Changes to civic score formula

## Development Setup

```bash
# Clone
git clone https://github.com/politic-in/core.git
cd core

# Install dependencies
go mod download

# Run tests
go test ./...

# Generate SDK (if modifying proto)
./scripts/generate-sdks.sh
```

## Package Guidelines

### Code Style

- Follow standard Go conventions (`go fmt`, `go vet`)
- Keep functions small and focused
- Add comments for exported functions
- Use meaningful variable names

### Testing

- All new code should have tests
- Run `go test ./...` before submitting
- Aim for >80% coverage on new code

### Commits

- Use clear, descriptive commit messages
- Reference issues when applicable (`Fixes #123`)
- Keep commits atomic (one logical change per commit)

## Package-Specific Notes

### booth-matching

- Test with actual Indian booth names
- Consider regional language variations (Hindi, Tamil, Telugu, etc.)
- Phonetic matching should handle transliteration differences

### data

- Geographic data should reference official sources (ECI, Census)
- Boundary files should be in GeoJSON format
- Include source attribution for data corrections

### anonymization

- Privacy changes require extra review
- Document any changes to k-anonymity or differential privacy
- Include mathematical justification for algorithm changes

### civic-score

- Score formula changes affect all users
- Propose changes as issues first for community discussion
- Include rationale for point value changes

## Data Contributions

For geographic data corrections:

1. Identify the source of truth (ECI website, Census data)
2. Include evidence (screenshot, official document)
3. Test that changes don't break existing functionality

## Questions?

- Open a GitHub issue for technical questions
- Email tsksandeep11@gmail.com for other inquiries

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.
