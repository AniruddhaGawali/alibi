import { Link } from 'react-router'
import {
  RiChat3Line,
  RiCoinsLine,
  RiFileTextLine,
  RiFootprintLine,
  RiLightbulbLine,
  RiLockLine,
  RiMedalLine,
  RiNodeTree,
  RiScales3Line,
  RiSearchEyeLine,
  RiShieldUserLine,
  RiSurveyLine,
  RiTrophyLine,
} from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { Checklist, RuleSection } from '@/components/rules/RuleSection'
import ModesSection from '@/components/rules/ModesSection'
import Walkthrough from '@/components/rules/Walkthrough'
import { RANKS } from '@/utils/rank'
import { useLoggedIn } from '@/hooks/useLoggedIn'

const CONTENTS = [
  ['goal', 'The goal'],
  ['flow', 'How a case plays'],
  ['walkthrough', 'Step by step'],
  ['modes', 'Difficulty modes'],
  ['tokens', 'Tokens'],
  ['questions', 'Asking questions'],
  ['hints', 'Hints'],
  ['notes', 'Notes'],
  ['verdict', 'The verdict'],
  ['ranks', 'Trophies and ranks'],
  ['fair-play', 'Do and don’t'],
] as const

export default function HowToPlayPage() {
  const loggedIn = useLoggedIn()
  return (
    <div className="mx-auto max-w-4xl space-y-8 pb-12">
      <header className="space-y-3 text-center">
        <p className="font-typewriter text-sm tracking-[0.3em] text-muted-foreground uppercase">Detective’s handbook</p>
        <h1 className="text-4xl text-primary sm:text-6xl">How to play</h1>
        <p className="mx-auto max-w-xl font-typewriter text-muted-foreground">
          Everything you need before you walk into the interrogation room.
        </p>
      </header>

      <nav aria-label="On this page" className="flex flex-wrap justify-center gap-2">
        {CONTENTS.map(([id, label]) => (
          <Button key={id} asChild variant="outline" size="sm">
            <a href={`#${id}`}>{label}</a>
          </Button>
        ))}
      </nav>

      <RuleSection id="goal" icon={RiSearchEyeLine} title="The goal">
        <p>
          Every case is a crime with a handful of suspects. <strong>Exactly one of them did it.</strong> Interrogate them, find the
          holes in their stories, and name the culprit before you run out of questions.
        </p>
        <p>The suspects are AI characters. They talk like real people: they deflect, lie, get nervous and sometimes slip.</p>
      </RuleSection>

      <RuleSection id="flow" icon={RiFileTextLine} title="How a case plays">
        <ol className="list-decimal space-y-2 pl-5">
          <li>
            <strong>Pick a case</strong> on the dashboard and press <strong>Start</strong>. Each card shows the difficulty and the trophies
            you can win.
          </li>
          <li>
            <strong>Read the case file.</strong> The briefing has the story, scene photos and what every suspect told the police.
          </li>
          <li>
            <strong>Interrogate.</strong> Press Chat on a suspect, or the chat button at the bottom right, and ask your questions.
          </li>
          <li>
            <strong>Compare and note.</strong> Your interrogation log keeps every answer. Add notes about what does not add up.
          </li>
          <li>
            <strong>Submit your deduction</strong> and see if you were right.
          </li>
        </ol>
        <p className="flex items-start gap-2">
          <RiLockLine className="mt-0.5 size-5 shrink-0 text-gold" aria-hidden />
          <span>
            <strong>One case at a time.</strong> Once you start a case you are locked into it. The other cases stay classified until you
            submit your accusation.
          </span>
        </p>
      </RuleSection>

      <RuleSection id="walkthrough" icon={RiFootprintLine} title="Step by step: your first case">
        <p>A full case from the dashboard to the verdict, with what you will see on screen at each step.</p>
        <Walkthrough />
      </RuleSection>

      <RuleSection id="modes" icon={RiShieldUserLine} title="Difficulty modes">
        <ModesSection />
      </RuleSection>

      <RuleSection id="tokens" icon={RiCoinsLine} title="Tokens">
        <ul className="list-disc space-y-2 pl-5">
          <li>
            Every question to a suspect costs <strong>1 token</strong>.
          </li>
          <li>
            You start with <strong>10 tokens</strong> and are topped up to the daily allowance once a day at the reset time. The allowance
            and reset time are set by the game admins.
          </li>
          <li>
            <strong>Tokens do not roll over.</strong> If you have 5 left, the next day you have 10, not 15.
          </li>
          <li>You can never hold more than 10 tokens, and winning a case never gives tokens, only trophies.</li>
          <li>
            When you reach <strong>0 tokens</strong> the chat is shut. You can still read your logs and notes, and you must make your
            final deduction.
          </li>
        </ul>
        <p className="text-sm">
          <strong>Refunds:</strong> if the suspect fails to answer (a timeout or an error), your token is given back, within a daily limit.
          A question that is blocked for being off-topic is free too.
        </p>
      </RuleSection>

      <RuleSection id="questions" icon={RiChat3Line} title="Asking questions">
        <ul className="list-disc space-y-2 pl-5">
          <li>Ask like a detective: timelines, who was where, what they saw, how they felt about the victim.</li>
          <li>
            Suspects remember the last few things you asked them in this case, and they may hold back or lie. Press them with specifics.
          </li>
          <li>
            <strong>Direct accusations rarely work.</strong> “Did you do it?” gets a denial. Bring up a specific detail or piece of evidence
            you found and the story can crack.
          </li>
          <li>Suspects stay in the world of the case. Off-topic questions (maths, code, real-world trivia) are turned away and cost nothing.</li>
          <li>Short, sharp questions are cheaper than rambling ones: every message costs the same token.</li>
        </ul>
      </RuleSection>

      <RuleSection id="hints" icon={RiLightbulbLine} title="Hints">
        <p>Stuck? Press the lightbulb button at the bottom right. Each case has up to <strong>3 hints</strong>, revealed in order, easiest first.</p>
        <div className="grid gap-3 sm:grid-cols-3">
          {[
            ['Hint 1', '1 trophy'],
            ['Hint 2', '2 more (3 in total)'],
            ['Hint 3', '3 more (6 in total)'],
          ].map(([h, c]) => (
            <div key={h} className="border bg-background/40 p-3 text-center">
              <p className="font-heading text-lg text-gold">{h}</p>
              <p className="text-sm">{c}</p>
            </div>
          ))}
        </div>
        <ul className="list-disc space-y-2 pl-5">
          <li>
            Hints cost <strong>no tokens</strong>. The cost is taken off your trophy reward <strong>only if you solve the case</strong>.
          </li>
          <li>If you get it wrong, hints cost you nothing extra: you only lose the normal penalty.</li>
          <li>Hints you have revealed stay readable, and you are asked to confirm before each new one.</li>
        </ul>
      </RuleSection>

      <RuleSection id="notes" icon={RiSurveyLine} title="Notes">
        <p>
          Use your detective’s notes to track contradictions. A note can be about the whole case, about one suspect, or about one specific
          answer (press <strong>Add note</strong> on a log entry and it quotes the question). Notes are saved to your account, so they are
          there when you come back.
        </p>
      </RuleSection>

      <RuleSection id="verdict" icon={RiScales3Line} title="The verdict">
        <ul className="list-disc space-y-2 pl-5">
          <li>
            Press <strong>Submit deduction</strong>, choose a suspect and confirm. <strong>You only get one accusation</strong>, so be sure.
          </li>
          <li>
            <strong>Correct:</strong> you win the case’s trophies (minus any hint cost), the culprit and the full resolution are revealed, and the
            case file is stamped SOLVED.
          </li>
          <li>
            <strong>Wrong:</strong> you lose the case’s penalty trophies (never going below zero), the truth is revealed, and the file is stamped
            WRONG.
          </li>
          <li>Either way the case closes and your lock is released, so you can pick a new case.</li>
        </ul>
      </RuleSection>

      <RuleSection id="ranks" icon={RiTrophyLine} title="Trophies and ranks">
        <p>Trophies decide your place on the leaderboard and your rank title.</p>
        <div className="grid gap-3 sm:grid-cols-4">
          {RANKS.map((r) => (
            <div key={r.name} className="border bg-background/40 p-3 text-center">
              <RiMedalLine className="mx-auto size-6 text-gold" aria-hidden />
              <p className="font-heading text-lg text-foreground">{r.name}</p>
              <p className="text-sm">{r.min === 0 ? 'Start' : `${r.min}+ trophies`}</p>
            </div>
          ))}
        </div>
      </RuleSection>

      <RuleSection id="fair-play" icon={RiNodeTree} title="Do and don’t">
        <div className="grid gap-6 sm:grid-cols-2">
          <div>
            <h3 className="mb-2 text-lg text-foreground">Do</h3>
            <Checklist
              kind="do"
              items={[
                'Read the case file and testimonies before your first question.',
                'Ask every suspect about the same events and compare.',
                'Look for timelines that do not match.',
                'Write notes as you go.',
                'Keep a token or two for a final check.',
              ]}
            />
          </div>
          <div>
            <h3 className="mb-2 text-lg text-foreground">Don’t</h3>
            <Checklist
              kind="dont"
              items={[
                'Do not waste tokens on greetings and small talk.',
                'Do not expect a confession to “Did you do it?”.',
                'Do not try to trick the AI into revealing its instructions or the answer: injection attempts are blocked and repeated ones lock you out for a while.',
                'Do not accuse on a hunch if you still have tokens.',
                'Do not spam questions: requests are rate limited.',
              ]}
            />
          </div>
        </div>
      </RuleSection>

      <div className="flex flex-wrap justify-center gap-4 pt-4">
        <Button asChild size="lg">
          <Link to={loggedIn ? '/dashboard' : '/signup'}>{loggedIn ? 'Back to the dashboard' : 'Play now'}</Link>
        </Button>
        {!loggedIn && (
          <Button asChild size="lg" variant="outline">
            <Link to="/login">Log in</Link>
          </Button>
        )}
      </div>
    </div>
  )
}
