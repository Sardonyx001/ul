# ul: a minimal, fast and secure URL shortener

## why?

This is a hobby project for now, but I hope to make it a useful tool for myself and others in the future.
The main goal is to create a simple, fast, and secure URL shortener that can be used by anyone.

## what?

This API supports the following endpoints:

- `POST /s`: Shortens a given URL.
- `GET /:shortened`: Redirects to the original URL based on the shortened version
- `GET /:shortened/stats`: Returns statistics about the shortened URL
- `GET /:shortened/qr`: Returns a QR code for the shortened URL.

## how?

### run with go

```bash
go run main.go

# or live-reload with air
go install github.com/cosmtrek/air@latest
air

# or build and run with docker
docker build -t ul .
docker run -p 7000:7000 ul
```

## todo

- [x] Implement URL shortening logic (`POST /s` endpoint)
- [x] Add URL redirect functionality (`GET /:shortened` endpoint)
- [x] Create statistics tracking (`GET /:shortened/stats` endpoint)
- [x] Generate QR codes for shortened URLs (`GET /:shortened/qr` endpoint)
- [x] Add database integration for URL storage
- [ ] Implement rate limiting and security measures
- [x] Add URL validation and sanitization
- [ ] Create comprehensive test suite
- [ ] Add Docker configuration
- [ ] Set up CI/CD pipeline
- [x] Add logging and monitoring
- [ ] Create API documentation
