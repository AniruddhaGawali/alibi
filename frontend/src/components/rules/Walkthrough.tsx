import type { ReactNode } from 'react'
import GuideScreenshot from './GuideScreenshot'

interface Step {
  title: string
  body: ReactNode
  /** File name under guide/ in the asset bucket. */
  image: string
  alt: string
  caption: string
}

const STEPS: Step[] = [
  {
    title: 'Pick a case on the dashboard',
    image: '01-dashboard.webp',
    alt: 'The dashboard with case cards, their difficulty, trophy reward and Start button',
    caption: 'Each card shows the difficulty and the trophies at stake. Locked cards stay classified while you are in another case.',
    body: (
      <>
        <p>
          After you log in you land on the <strong>dashboard</strong>: your tokens, trophies and rank on top, the case cards below. Read
          a card’s synopsis, then press <strong>Start</strong>.
        </p>
        <p>Starting locks you into that case until you submit an accusation, so pick one you have time for.</p>
      </>
    ),
  },
  {
    title: 'Read the case file',
    image: '02-case-file.webp',
    alt: 'The case file page with the scene photos on the left and the case story on the right',
    caption: 'Scene photos on one side, the story of the crime on the other.',
    body: (
      <>
        <p>
          The case page opens with the <strong>case file</strong>: the scene photos and the full story of the crime. Read it all before
          you spend a token. Times, places and objects mentioned here are what you will check the suspects against.
        </p>
      </>
    ),
  },
  {
    title: 'Meet the suspects',
    image: '03-suspects.webp',
    alt: 'Suspect cards with portrait, name, role, first statement and a Chat button',
    caption: 'Investigators also see each suspect’s public knowledge under the first statement.',
    body: (
      <>
        <p>
          Below the case file are the <strong>suspect cards</strong>: portrait, name, role and the first statement each one gave the police.
          In <strong>Investigator</strong> mode a card also shows the suspect’s <strong>public knowledge</strong>, background the whole
          town already knows.
        </p>
        <p>Clicking a card (not the button) filters your interrogation log to that suspect.</p>
      </>
    ),
  },
  {
    title: 'Open the interrogation',
    image: '04-chat-open.webp',
    alt: 'The interrogation panel open on the right, with the suspect picker, the token counter and the question box',
    caption: 'The token counter sits in the top right of the panel. Switch suspects with the picker.',
    body: (
      <>
        <p>
          Press <strong>Chat</strong> on a suspect card, or the round <strong>chat button</strong> at the bottom right of the screen. The
          interrogation panel slides in from the right with the suspect’s first statement at the top.
        </p>
        <ul className="list-disc space-y-1 pl-5">
          <li>Use the picker at the top to switch to another suspect without closing the panel.</li>
          <li>The counter in the top right shows the tokens you have left.</li>
        </ul>
      </>
    ),
  },
  {
    title: 'Ask your questions',
    image: '05-chat-answer.webp',
    alt: 'A question and the suspect’s typed-out answer in the chat, with a detective tip under the question box',
    caption: 'Answers type themselves out; click to skip. Investigators get a tip under the box.',
    body: (
      <>
        <p>
          Type a question and press <strong>Enter</strong> (Shift+Enter for a new line) or the send button. One question costs{' '}
          <strong>1 token</strong>, taken the moment you send it. Questions can be up to 500 characters.
        </p>
        <ul className="list-disc space-y-1 pl-5">
          <li>Ask about one thing at a time: where they were, when, who saw them, what they heard or smelled.</li>
          <li>Quote what another suspect told you and watch how they react.</li>
          <li>Off-topic questions are turned away and cost nothing. If the suspect fails to answer, the token comes back.</li>
          <li>At 0 tokens the box locks: it is time to make your deduction.</li>
        </ul>
      </>
    ),
  },
  {
    title: 'Review the log and take notes',
    image: '06-log-notes.webp',
    alt: 'The interrogation log under the suspects, with an Add note button on an answer and the notes beside it',
    caption: 'Every answer is kept in the log. Pin a note to one answer and it quotes the question.',
    body: (
      <>
        <p>
          Every answer is saved in the <strong>interrogation log</strong> below the suspects. Filter it by suspect, and press{' '}
          <strong>Add note</strong> on an entry to write down what does not add up. Notes are kept on your account.
        </p>
      </>
    ),
  },
  {
    title: 'Stuck? Use a hint',
    image: '07-hint.webp',
    alt: 'The hint dialog asking to confirm before revealing the next hint, with its trophy cost',
    caption: 'The lightbulb button sits above the chat button. You are asked before each hint.',
    body: (
      <>
        <p>
          The <strong>lightbulb</strong> above the chat button reveals up to 3 hints, in order. They cost no tokens; 1, then 2, then 3
          trophies come off your reward, and only if you win.
        </p>
      </>
    ),
  },
  {
    title: 'Name the culprit',
    image: '08-accuse.webp',
    alt: 'The accusation dialog with every suspect on a folder and one selected in red, with the Submit Deduction button',
    caption: 'Select one suspect; they turn red and get a paper clip. Then submit.',
    body: (
      <>
        <p>
          When you are sure, press <strong>Submit Deduction</strong> under the case title. Every suspect is laid out on a folder:
          click the one you accuse (they turn red), then press <strong>Submit Deduction</strong> at the bottom of the dialog.
        </p>
        <p>
          <strong>You get one accusation per case</strong> and it cannot be taken back. Check your notes first, and use a last token to
          confirm a doubt if you have one.
        </p>
      </>
    ),
  },
  {
    title: 'The verdict',
    image: '09-verdict.webp',
    alt: 'The case outcome with the SOLVED stamp, the trophy change and the full resolution',
    caption: 'Right or wrong, the truth and the resolution are revealed and the case closes.',
    body: (
      <>
        <p>
          The case file is stamped <strong>SOLVED</strong> or <strong>WRONG</strong>, your trophies change, and the culprit and the full
          story are revealed. The lock is released, so you can head back to the dashboard and take the next case.
        </p>
      </>
    ),
  },
]

/** The illustrated, step-by-step walkthrough of one case on the handbook page. */
export default function Walkthrough() {
  return (
    <ol className="space-y-10">
      {STEPS.map((s, i) => (
        <li key={s.title} className="grid items-start gap-5 md:grid-cols-2">
          <div className="space-y-3">
            <h3 className="flex items-baseline gap-3 text-xl text-foreground">
              <span className="font-heading text-3xl text-gold">{i + 1}</span>
              {s.title}
            </h3>
            {s.body}
          </div>
          <GuideScreenshot file={s.image} alt={s.alt} caption={s.caption} />
        </li>
      ))}
    </ol>
  )
}
