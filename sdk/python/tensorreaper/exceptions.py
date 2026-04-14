"""Custom exceptions for the TensorReaper SDK."""


class TensorReaperError(Exception):
    """Base exception for all TensorReaper SDK errors."""

    def __init__(self, message: str, status_code: int | None = None, detail: str | None = None):
        self.status_code = status_code
        self.detail = detail
        super().__init__(message)


class AuthenticationError(TensorReaperError):
    """Raised when authentication fails (401)."""

    def __init__(self, message: str = "Authentication failed", detail: str | None = None):
        super().__init__(message, status_code=401, detail=detail)


class AuthorizationError(TensorReaperError):
    """Raised when authorization fails (403)."""

    def __init__(self, message: str = "Authorization denied", detail: str | None = None):
        super().__init__(message, status_code=403, detail=detail)


class NotFoundError(TensorReaperError):
    """Raised when a resource is not found (404)."""

    def __init__(self, resource_type: str, name: str):
        self.resource_type = resource_type
        self.name = name
        super().__init__(
            f"{resource_type} '{name}' not found",
            status_code=404,
            detail=f"{resource_type} '{name}' not found",
        )


class ValidationError(TensorReaperError):
    """Raised when request validation fails (400)."""

    def __init__(self, message: str = "Validation error", detail: str | None = None):
        super().__init__(message, status_code=400, detail=detail)


class RateLimitError(TensorReaperError):
    """Raised when rate limit is exceeded (429)."""

    def __init__(self, message: str = "Rate limit exceeded", detail: str | None = None):
        super().__init__(message, status_code=429, detail=detail)


class ServerError(TensorReaperError):
    """Raised when the server returns a 5xx error."""

    def __init__(self, message: str = "Server error", status_code: int = 500, detail: str | None = None):
        super().__init__(message, status_code=status_code, detail=detail)


class TimeoutError(TensorReaperError):
    """Raised when an operation times out (e.g., waiting for job completion)."""

    def __init__(self, message: str = "Operation timed out"):
        super().__init__(message)


class ConfigurationError(TensorReaperError):
    """Raised when SDK configuration is invalid or incomplete."""

    def __init__(self, message: str):
        super().__init__(message)
