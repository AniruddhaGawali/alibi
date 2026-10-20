import EconomySection from '@/components/landing/EconomySection'
import FinalCta from '@/components/landing/FinalCta'
import HowToPlay from '@/components/landing/HowToPlay'
import LandingFooter from '@/components/landing/LandingFooter'
import LandingHero from '@/components/landing/LandingHero'
import LandingLeaderboard from '@/components/landing/LandingLeaderboard'
import LandingTopBar from '@/components/landing/LandingTopBar'
import ModesBand from '@/components/landing/ModesBand'
import RankLadder from '@/components/landing/RankLadder'

export default function LandingPage() {
  return (
    <div className="relative">
      <LandingTopBar />
      <LandingHero />
      <HowToPlay />
      <ModesBand />
      <EconomySection />
      <RankLadder />
      <div id="leaderboard" className="scroll-mt-8">
        <LandingLeaderboard />
      </div>
      <FinalCta />
      <LandingFooter />
    </div>
  )
}
