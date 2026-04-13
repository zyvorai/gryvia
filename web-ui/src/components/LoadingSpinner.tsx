export default function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center py-12">
      <div className="text-center">
        <div className="w-8 h-8 rounded-full animate-spin mx-auto mb-3"
          style={{
            border: '2px solid rgba(192,204,224,0.1)',
            borderTopColor: '#d4764e',
            boxShadow: '0 0 12px rgba(212,118,78,0.2)',
          }}
        />
        <div className="text-sm text-[#5a7a9e]">Loading...</div>
      </div>
    </div>
  )
}
