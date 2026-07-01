export function LoadingState({ message = 'Loading data…' }: { message?: string }) {
  return (
    <div className="state-box">
      <h4>{message}</h4>
      <p>Please wait while records are retrieved.</p>
    </div>
  )
}
