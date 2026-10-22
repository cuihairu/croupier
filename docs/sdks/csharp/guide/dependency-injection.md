# Dependency Injection

The C# SDK integrates well via dependency injection in ASP.NET Core or background services.

## Recommendations

- Register `CroupierClient` as a singleton or with a controlled lifetime
- Keep handlers decoupled from business services
- Inject configuration via `IOptions<T>`
