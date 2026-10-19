import Leaderboard from '@/components/dashboard/Leaderboard'
import Reveal from './Reveal'

export default function LandingLeaderboard() {
  return (
    <section className="landing-band border-y py-16">
      <Reveal className="mx-auto max-w-4xl px-6">
        <Leaderboard centered />
      </Reveal>
    </section>
  )
}
