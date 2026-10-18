/** Steel paper clip holding a case file together; parent must be `relative`. */
export default function PaperClip() {
  return (
    <svg className="paper-clip" viewBox="0 0 34 78" fill="none" aria-hidden>
      <defs>
        <linearGradient id="clip" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#f2f2f2" />
          <stop offset=".5" stopColor="#9aa0a6" />
          <stop offset="1" stopColor="#d6d9dc" />
        </linearGradient>
      </defs>
      {/* outer loop, inner loop and the tail of a gem clip */}
      <path d="M9 62V14a8 8 0 0 1 16 0v46a12 12 0 0 1-24 0V20" stroke="#4b4f54" strokeWidth="4.2" strokeLinecap="round" />
      <path d="M9 62V14a8 8 0 0 1 16 0v46a12 12 0 0 1-24 0V20" stroke="url(#clip)" strokeWidth="2.6" strokeLinecap="round" />
      <path d="M17 52V18a2 2 0 0 1 4 0" stroke="url(#clip)" strokeWidth="2.4" strokeLinecap="round" />
    </svg>
  )
}
