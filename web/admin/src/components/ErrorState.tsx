export function ErrorState({
  message,
  onRetry,
}: {
  message: string
  onRetry?: () => void
}) {
  return (
    <div className="state-box error">
      <h4>Unable to load data</h4>
      <p>{message}</p>
      {onRetry && (
        <p style={{ marginTop: '1rem' }}>
          <button type="button" className="btn btn-secondary" onClick={onRetry}>
            Try again
          </button>
        </p>
      )}
    </div>
  )
}
