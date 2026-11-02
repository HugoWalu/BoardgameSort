# Board Game Night Planner — Project Plan

A private, phone-first tool for one board game group: shared shelf, "what fits tonight",
ranked-choice voting on the night's plan, and (later) play history that makes the planner smarter.

---

## 1. Locked decisions

| Decision | Choice |
| --- | --- |
| Audience | Just this group. One group, private. Multi-tenant columns from day one, no multi-group UI. |
| Weekly hook | Game-night voting + Tonight mode |
| Auth | Real accounts, **invite links pasted in the group chat**. No SMTP. |
| Poll tally | **Hidden until the host closes** (or auto-close timer). Reveal is the payoff. |
| Voting method | Ranked choice (instant runoff), truncated ballots allowed |
| Poll options | **Plans**, not games. A single-game poll is a 1-slot plan. |
| Planner output | 2–3 preset shapes, user ranks the candidate plans |
| Campaign/legacy | Not in scope. Revisit later. |
| Stack | Next.js + TypeScript + Tailwind + Postgres. Details in §10. |
| Hosting | Vercel Hobby + Supabase free Postgres. Free `*.vercel.app` subdomain for now. Details in §10.1. |

### Product thesis

The shelf is one-time setup. The thing opened every week is:
**"we're 5 people, we have 3 hours, what are we playing, and does everyone agree?"**

Every step in that loop must take under ~10 seconds. If voting requires a signup form, it dies.

---

## 2. Non-goals

- Public signup, discovery, or social features
- Buying recommendations (M3 at the earliest, and only as gap analysis)
- Websockets / true realtime (3s polling is invisible to users)
- Desktop-first table UI
- Batch BGG library sync in M1
- Native apps

---

## 3. Milestones

### M1 — the loop (build this first)

Invite-link auth, shelf with copies and lending, one-slot Tonight shortlist, ranked-choice poll
with hidden tally and IRV reveal, deployed to a real HTTPS URL.

**Definition of done:** used for one real game night, and notes taken on every point of friction.

### M2 — the planner

Slot-based plans, preset shapes, teach and slowdown models, editable slots, attendee windows,
plan-level polls.

### M3 — history and intelligence

Play logging with measured durations and teach times, stats, taste profiles, optional buying gaps.

Ordered, buildable steps for all three are in §13.

---

## 4. Core mechanics

### 4.1 Player count is a verdict, not a number

Never sort by `max_players`. Use the BGG suggested-players poll:

```
playerCount → { best: [4], recommended: [3,4,5], avoid: [2,6] }
```

- `best` scores highest, `recommended` next, "within range" is legal but weak
- Unknown data degrades gracefully: fall back to the legal range and label it
  **"player count unverified"** — do not pretend
- A shortlist that confidently recommends a 2-player game to 6 people is worse than no shortlist

### 4.2 Time is not game time

Every slot estimate is:

```
est_minutes    = effective_play + overhead
effective_play = playing_time * (1 + slowdown)            // §4.2.2
overhead       = max(teach, setup) + 0.5 * min(teach, setup)   // §4.2.1
setup          = 3 (base) | 5-10 (expansions, inserts)
```

Setup and teach **overlap** — someone sets up while you explain. Charging both in full
double-counts, hence the `max + half the smaller` rule.

A 6-hour plan that sums BGG `playingtime` overruns by an hour. Always show the breakdown:

```
Wingspan  70 play +11 slow (1 new)  +8 teach  +3 setup  = 92 min
```

Breaks are slots, not gaps. A session ≥ 240 min gets a 15-minute break; ≥ 300 min gets
a 40-minute dinner slot.

#### 4.2.1 Teach time is computed per slot, not per game

Teach is **not always included**, and it is not a static game property. It depends on who is
sitting at the table. Three cases:

| Case | Cost |
| --- | --- |
| **Learn** — at least one attendee has never played it | Full teach |
| **Refresh** — everyone has played, but it was months ago | A fraction of full teach |
| **Warm** — everyone played it recently | ~0; render nothing, not "+0 min" |

Never treat "not played in 8 months" as "never played". A refresh is 2–5 minutes; teaching
Concordia is 17.

**Full teach from weight and length** (weight carries most of the signal; length adds a little
because long games have more subsystems):

```
teach_full = 2 * weight^1.8 * (0.75 + playing_time / 240)
```

| Game | Weight | Length | Teach |
| --- | --- | --- | --- |
| Just One | 1.1 | 20 | 2 min |
| Codenames | 1.3 | 15 | 3 min |
| Carcassonne | 1.9 | 40 | 6 min |
| Catan | 2.3 | 60 | 9 min |
| Wingspan | 2.4 | 60 | 10 min |
| Concordia | 3.0 | 100 | 17 min |
| Brass: Birmingham | 3.9 | 120 | 28 min |
| Twilight Imperium | 4.3 | 240 | 46 min |

Display rounded to the minute; keep the float for the maths. Missing weight → assume 2.5 and
mark the estimate low-confidence.

**Familiarity ladder — the single shared definition**, used by both teach (below) and
slowdown (§4.2.2). Per person, per game:

```
familiarity(user, game)
  0 plays                        → 1.00
  1 play                         → 0.60
  2 plays                        → 0.30
  ≥3 plays, played < 180 days    → 0.00
  ≥3 plays, played ≥ 180 days    → 0.30   (they'll ask rules questions)
```

Teach takes the **minimum** across attendees — the least experienced person sets the floor.
Slowdown takes the **sum** — every novice at the table contributes. Four veterans plus one
person who played it once costs 60% of a full teach, not 100% and not zero.

```
plays_min = min(familiarity(user, game) for user in attendees)
```

**Refresh from recency**, using the attendee who last played it *longest ago*:

```
age = max(days_since_last_play[user][game] for user in attendees)

age ≥ 180d → 0.40 × teach_full  (cap 10 min)
age ≥  60d → 0.25 × teach_full  (cap 6 min)
age ≥  21d → 0.10 × teach_full  (cap 3 min)
else       → 0
```

Combine as:

```
teach = max(teach_full * plays_min, refresh)
```

The ladder covers partial knowledge; the refresh term covers "everyone knows it, but it's
been months". Never both at once.

**Scaling with table size** — more people means more side questions:

```
teach *= 1 + 0.08 * (attendees - 2)
```

**Overrides**: `games.teach_override_min` wins over the formula (player aids, asymmetric
factions, one person who always teaches it). Per-game, not per-session.

#### 4.2.2 New players slow the whole game down

A separate cost from teaching, and the one that's easy to miss. **Teach is upfront and
one-time; slowdown is distributed across every turn** — "wait, can I do that again?" on
turn 9. Plans that budget only for the teach run long every time a new player sits down.

Scale the play time rather than adding a fixed number, since a 3-hour game slows more than
a 20-minute one:

```
slowdown = 0.5 * (1 - e^(-0.55 * novelty_sum)) * (0.4 + 0.2 * weight)
capped at 0.60

effective_play = playing_time * (1 + slowdown)
```

**`novelty_sum` uses that same familiarity ladder** — summed across attendees, since every
novice at the table contributes:

```
novelty_sum = Σ familiarity(attendee, game)      // §4.2.1, range 0..n
```

Fractional sums mean a rusty veteran costs less than a total newcomer, and the exponential
makes the fourth new player hurt less than the first — one new player slows the table, four
don't slow it four times as much.

Weight scaling — `0.4 + 0.2 * weight`, so party games barely slow down and heavy games bite:

| Weight | Scale |
| --- | --- |
| 1.1 | 0.62 |
| 2.0 | 0.80 |
| 3.0 | 1.00 |
| 3.9 | 1.18 |
| 4.4 | 1.28 |

Worked examples:

| Situation | Base | Slowdown | Effective |
| --- | --- | --- | --- |
| Wingspan, 4p, 1 new | 70 | +19% | 83 |
| Wingspan, 4p, 3 new | 70 | +36% | 95 |
| Concordia, 4p, 1 played once | 100 | +14% | 114 |
| Brass, 4p, 1 new | 120 | +25% | 150 |
| Catan, 4p, all new | 75 | +38% | 104 |
| Just One, 6p, 2 new | 20 | +21% | 24 |
| TI4, 4p, all new | 240 | +56% | 374 |

Cap at +60% so a pathological composition can't produce a 6-hour slot for a 2-hour game.

**Every consumer must use `effective_play`, not `playing_time`:**

- The DP filler (§4.5) budgets on effective time, or it schedules five games that don't fit —
  and "the plan ends when it says it ends" is the whole promise
- `fits_time` in the scorer (§4.3) sees the inflated duration, which naturally prefers games
  the group already knows when the window is tight — useful emergent behaviour, not a bug
- Plan comparison gains a second honest number:
  `2 new games · 55 min teach · +30 min slow play`

**This one also fixes itself with data.** `plays.duration_min` *already contains* the
slowdown — a logged 4-player Wingspan with one new player took 85 minutes, full stop.
So: log `plays.new_players` (derived automatically from the `knows` set at log time, no UI),
bucket measured medians by `(game, player_count, new_players > 0)`, and once a bucket has
2+ plays use the median and drop the formula for that case.

Two things not to conflate:

- **Teach ≠ slowdown.** A game can have a 5-minute teach and a 40% slowdown (Splendor with
  three new players), or a 45-minute teach and no slowdown at all (everyone has played TI4).
- **New to the hobby ≠ new to this game.** A 200-game veteran who hasn't played Wingspan
  slows down far less than a first-timer. That needs a per-person skill flag — skip until
  the plan is proven.

#### 4.2.3 Where the inputs come from

- **M1/M2, no play history**: default to **full teach**. Conservative — an overrun ruins a
  plan, an extra 10 minutes of slack doesn't. Mark it low-confidence in the "why" breakdown.
- **M2 interim**: a `knows` set per game (tick which members have played it on the shelf).
  Ten minutes of tapping for a 20–40 game shelf, and it immediately gives the
  learn-vs-warm split without any logging.
- **M3**: real `plays` replace it. Record `plays.teach_minutes` on first plays and the app
  learns *your group's* number, which varies hugely by teacher. "We teach Brass in 22
  minutes, not 28" beats any formula.

#### 4.2.4 Interaction with attendee windows

Teach is computed for who is present **at slot start**. Someone arriving at 8:30 misses the
Concordia teach — flag it on the slot: *"Dan arrives 8:30, needs a catch-up"* and add ~5 min,
or reorder the plan so the teach happens when everyone is there.

### 4.3 Scoring

Keep it rules-based and explainable. Weights live in one config object; no weights UI.

```
score = +2.0 * fits_players      // best >> recommended > legal range > out
        +1.5 * fits_time         // vs est_minutes (§4.2); over budget ≈ 0
        +1.0 * fits_energy       // distance from target weight
        +1.0 * staleness         // never played / months since last
        +1.5 * group_rating
        +1.0 * attendee_taste
        -1.0 * played_recently(<=14d)
        -∞   * vetoed_by_attendee
```

`fits_time` compares against **`est_minutes`**, never raw `playingtime`: a 120-minute game
with three new players is effectively 160 and must score as such.

Where the two unbacked terms come from:

- **`group_rating`** — mean of submitted 1–10 ratings for the game (`ratings` table, §7).
  No ratings → **0**, not a penalty. Absence is neutral.
- **`attendee_taste`** — an **M3** term. Score it as 0 in M1/M2 rather than shipping a weight
  with no data behind it.

Show the reasons as chips on each card:

> `best at 4 ✓ · 90 min ✓ · not played in 5 months · Sam ★`

Add a "why" expander showing each term's contribution. That is ~80% of perceived
intelligence for free, and it's how you debug a bad ranking.

Cap the shortlist at **5**. Beyond that, the group argues about the list instead of the games.

### 4.4 Plan shapes

Presets are fractions of the **game budget** — `total_minutes` minus break slots. Per-game
teach, setup and slow-play costs are priced when the DP evaluates each candidate (§4.5), never
subtracted up front. Deducting them from the budget *and* charging `est_minutes` bills teaching
twice.

Three presets, each a profile of slot durations as a share of that budget:

```
Compact    [0.08, 0.42, 0.38, 0.12]      fewer rules, deeper games
Varied     [0.08, 0.28, 0.26, 0.24, 0.14]
Party-ish  [0.15, 0.20, 0.20, 0.20, 0.15, 0.10]
```

Slot index determines role: first slot = lightest/opener, last = short/closer, middles = heaviest.

**6-hour example** (start 6:00pm, 4 players, one of whom is new to Concordia and The Crew):

```
6:00pm  Dinner / arrive                                                      40m
6:40    Opener   Just One       20 play, warm                    +3 setup     23m
7:03    Main 1   Concordia     121 play (+21%, 1 new)  +20 teach  +3 setup    143m
9:26    Break                                                                15m
9:41    Main 2   Orleans        90 play  +2 refresh (7wks)        +3 setup     94m
11:15   Closer   The Crew       23 play (+17%, 1 new)  +7 teach   +3 setup     32m
11:47   ends — 13 min slack before midnight
```

Teaching load for this plan: **2 new games · ~27 min teach · ~24 min slow play.** That second
number is what most planners silently omit.

The same engine with `total = 90` produces one slot. One engine, two entry points.

### 4.5 Filling slots: DP over (time, slot index)

```
dp[t][k] = best total score using k games in t minutes
item contribution = base_score(game) + shape_fit(game, k)
                    - penalties(duplicate, recently played, veto, mechanic repeat)
```

- O(slots × minutes × games). 360 × 8 × 200 ≈ 576k transitions. Instant.
- Time is always **`est_minutes`** (§4.2), never raw `playingtime` — including the teach,
  overhead and per-slot slow-play penalty. Budgeting on raw times is what produces plans
  that promise 11:40pm and deliver 12:50am.
- A **locked** slot subtracts its game and its time, then the DP re-runs on the remainder.
- Greedy is tempting but produces six 45-minute fillers because they "fit perfectly".
  The shape term prevents that; the DP makes it correct.
- Tie/equivalence: dedupe identical slot lists before showing presets.

### 4.6 Attendee windows

`attendees(session_id, user_id, from_time, to_time)`. The planner schedules around it:

- 5-player game while all 5 are present
- 2-player-only game after the others leave
- The heavy game **before** the person who leaves at 9:30
- Per-slot header: `players present: 4 ✓`

A plan that violates a window is flagged **"✗ invalid: Dan leaves at 8"** but stays votable.
Never silently drop options.

### 4.7 Ranking and counting

Poll options are frozen plan snapshots. Single-game polls are 1-slot plans rendered as game cards.

```
ballots.ranking jsonb   // ["option_c", "option_a"] in preference order, [] = abstain
```

Count server-side only. The client must never receive rankings before close.

Instant runoff:

```
active = all options except those vetoed by an attendee
ballots = submitted ballots mapped to their highest-ranked active option
loop:
  if an option has > 50% of non-exhausted ballots: winner
  eliminate the lowest
  redistribute its ballots to next active preference
  exhausted ballots leave the denominator
until one option remains
```

- Partial/truncated ballots are normal; exhaustion is not an error
- Elimination tie → highest planner score, and **say so in the reveal**
- All ballots exhausted → "the group is undecided, host's call". Never invent a winner
- Freeze the round-by-round result at close into `polls.result` so the reveal is stable

### 4.8 The ballot UI

Because the tally is hidden, the ballot is the voter's **only** feedback that a tap registered.
Misclicks are therefore expensive: the voter can't tell from a number whether they meant it.

**Order indicator — two layers, both required**

- **Numbered chip on each ranked row**: large filled circle, high contrast, `#1 / #2 / #3`.
  Ranked rows get a raised border; unranked stay flat. Never rely on color alone.
- **"Your order" strip** pinned above the action bar, wrapping (not scrolling):

  ```
  1 Concordia → 2 Skull → 3 Cascadia → 4 Just One
  ```

  The strip is what makes a five-item ballot verifiable in one glance instead of by
  scanning the list for scattered numbers.

**One tap, one meaning**

| Action | Result |
| --- | --- |
| Tap unranked row | Append to end |
| Tap ranked row | Remove from ranking |
| Up/down chevrons in the strip | Reorder |
| Tap `×` on a strip entry | Remove |

Reordering lives only in the strip, so a thumb aiming to reorder can't accidentally unrank,
and vice versa. One meaning per control is also what makes undo self-describing.

**Undo**

A persistent control in the bottom action bar — not a toast that expires:

```
↶ Undo  ranked Just One #4        [ Submit ballot · 4 ranked ]
```

- Covers every ballot mutation: rank, unrank, reorder, and veto toggle
- Full session history (bounded ~20 entries), not last-action-only — misclick cascades happen
- Disabled with an empty stack; `⌘Z` / `Ctrl+Z` on desktop
- Submitting clears the stack and swaps the bar to `Your ballot · Edit`
- No redo — it competes for the same corner for a rare need

**Draft persistence**

Every ballot change writes server-side with `submitted_at = null`, so a refresh, a dead
battery, or switching phones loses nothing. Only rows with `submitted_at IS NOT NULL` count
toward "4 of 6 ballots in" — nobody sees a half-finished ballot.

A submitted ballot stays editable until close, so undo is scoped to the editor, not the record.

### 4.9 Reveal

Round-by-round bars, eliminations in red, transfer arrows, revealed one step at a time.
For plan-level polls, include the slot diff between finalists:

> Round 2 — Compact 4 · Varied 3
> Varied eliminated; ballots transferred to Compact (2) and Party (1)
> **Winner: Compact** — swaps Orleans for Cascadia at 9:20

This is the only screen people screenshot. Give it real design time; the tally is already computed.

### 4.10 Session lifecycle

`draft → polling → chosen → in progress → logged`

Store a timestamp per transition. Night-of edits operate on a **copy** of the chosen plan's
slots so the poll history and reveal never mutate.

---

## 5. Screens (M1)

| Route | Purpose |
| --- | --- |
| `/join/:token` | Invite-link login, sets session cookie/account |
| `/` | Shelf: games, owner(s), lend status, last played |
| `/games/new`, `/games/:id` | Quick add (BGG id autofill), edit, copies, veto toggle |
| `/g/:group/members` | Names, invite link generation |
| `/tonight` | Constraints: headcount, minutes, energy, attendees, exclusions |
| `/tonight/result` | Scored top 5 with reason chips, "why" expander, "create poll" |
| `/poll/:id` | Ballot (voter view) or plan tabs + close button (host view) |
| `/poll/:id/reveal` | IRV rounds, winner, slot diff |

M2 adds the planner view (timeline with editable slots) between `/tonight` and `/poll`.

---

## 6. UX rules

- **Phone-first, dark mode.** Big tap targets, primary actions in thumb reach.
  It's used in a dim room with a drink in the other hand.
- **Ballot interaction is specified in §4.8** — chips, order strip, one-meaning taps, undo,
  server-side drafts. Not repeated here, so there's one place to change it.
- **No accounts for the vote** is not an option anymore, but the invite link must be one tap.
- **Participation, not preferences, while open.** Show `4 of 6 ballots in` and who hasn't voted.
- **Host is the bottle-neck** → default `closes_at` = now + 15 min, host can close early or extend.
- QR code rendered client-side from the poll URL. No extra service.

---

## 7. Data model (Postgres)

```sql
-- identity
users(id, name, email_nullable, created_at)
groups(id, name, created_at)
memberships(group_id, user_id, role, created_at)
invites(id, group_id, user_id, token, expires_at, used_at)

-- catalogue (BGG cache + manual entries)
games(
  id, title, year, bgg_id int unique null,
  min_players, max_players,
  best_players int[], recommended_players int[],
  playing_time, min_time, max_time,
  weight numeric(3,2),
  image_url, categories text[], mechanics text[],
  teach_override_min int null,
  source enum('bgg','manual'), updated_at
)

-- ownership
copies(id, game_id, owner_id, status, lent_to, lent_since, location)
-- status: owned | lent | previously_owned

vetoes(user_id, game_id, created_at, PRIMARY KEY (user_id, game_id))

ratings(group_id, game_id, user_id, score numeric(4,1), updated_at,
        PRIMARY KEY (group_id, game_id, user_id))
-- 1..10. group_rating = mean(score); absent ratings contribute nothing (0)

-- who has played what (M2 manual "knows" set; M3 derived from plays)
knows(user_id, game_id, source enum('manual','played'), created_at,
      PRIMARY KEY (user_id, game_id))

-- sessions and plans
sessions(id, group_id, start_at, total_minutes, headcount, state,
         chosen_plan_id, created_at)
attendees(session_id, user_id, from_time, to_time)

plans(id, session_id, preset, score, rationale jsonb, created_at)
slots(id, plan_id, position, kind, game_id, est_minutes, teach_minutes,
      duration_override, locked)
-- kind: opener | main | closer | filler | break | empty

-- polling
polls(id, session_id, status, closes_at, created_at, closed_at, result jsonb)
poll_options(id, poll_id, position, label, plan_snapshot jsonb)
ballots(id, poll_id, user_id, ranking jsonb, submitted_at, updated_at)
-- UNIQUE (poll_id, user_id)
-- submitted_at IS NULL  = draft, excluded from participation counts and tallies
-- ranking = []           = explicitly abstained

-- history (M3, create now with nullable fields)
plays(id, group_id, game_id, copy_id, played_on, duration_min,
      teach_minutes int null, new_players int, notes, created_by)
play_players(play_id, user_id, guest_name, position, score, winner)
```

Notes:

- `int[]` for player-count sets on Postgres. On SQLite use a join table — arrays break here.
- Indexes: `games(bgg_id)`, `slots(plan_id, position)`, `ballots(poll_id, user_id)`,
  `copies(game_id)`, `plays(group_id, played_on)`.
- `group_id` on everything from day one: a column now, a migration later.
- Create `slots`, `plans`, `knows`, `plays.duration_min`, `plays.teach_minutes`,
  `plays.new_players` and `games.teach_override_min` in M1 even though one-slot plans are
  all you make.
- Measured durations are bucketed for lookup as `(game, player_count, new_players > 0)` —
  that's the key the slowdown model (§4.2.2) is replaced by once a bucket has 2+ samples.

---

## 8. BGG integration

Endpoints (XML API2):

- `/xmlapi2/search?query=…&type=boardgame`
- `/xmlapi2/thing?id=13,12&stats=1` → `averageweight`, suggested-players poll
- `/xmlapi2/collection?username=…&stats=1` (M2/M3 batch import)
- `/xmlapi2/plays?username=…` (read-only backfill)

Practical requirements:

- Returns `202 Accepted` meaning "retry in a moment". Retry with backoff.
- **Never called while serving a read**: no page render, poll status, or planner run ever waits
  on BGG. Everything reads from the cached `games` row.
- The single synchronous call is the explicit add action: paste a BGG id/URL, fetch once with a
  ~5s timeout, store, done. On timeout or 202-past-retries, fall back to manual entry showing
  "couldn't reach BGG" — never a spinner with no exit.
- Refreshes are manual, or a later scheduled job. Never inline.
- BGG has **no write API** for plays. All play data lives in this app.
- Expansions return as separate `boardgameexpansion` items — skip them in M1, or they
  become duplicate shelf rows.
- Attribution and image hotlinking: verify current BGG API terms before launch; prefer
  caching thumbnails locally.
- Every BGG call sits behind one adapter interface so it can be swapped or stubbed.

---

## 9. Auth: invite links, no SMTP

1. Host generates `https://app/join/<token>` per person, valid 14 days, single use.
2. Paste into the group chat. One tap = logged in on that device.
3. Token is exchanged for a long-lived session cookie.
4. Same `token` mechanism later supports emailed magic links if ever wanted.

Fallback if a device is lost: host reissues a token.

---

## 10. Stack

- **Next.js (App Router) + TypeScript + Tailwind + shadcn/ui**
- **Postgres on Supabase** (free tier) — database only, **not** Supabase Auth
- **Vercel** for deploy
- **Custom invite-token auth** + signed httpOnly cookie (see §9). Supabase Auth would add a
  parallel identity system that the token flow already replaces.
- QR generation client-side (`qrcode`)
- Migrations: Drizzle (or Prisma) with a committed migrations folder, run against the
  **direct** connection — never from a serverless function

Fallback for self-hosting later: same app, Postgres in Docker, Better Auth.
Keep auth and BGG access behind thin interfaces so the swap is contained.

---

## 10.1 Hosting

Target: **Vercel Hobby** for the app + **Supabase free** Postgres. Zero cost, zero ops.
Load is trivial — ~50 requests a week plus a burst of 6 devices polling every 3s for
15 minutes — so no limits are close.

| Piece | Choice |
| --- | --- |
| App | Vercel Hobby, GitHub repo, auto-deploy `main` |
| Functions region | Same region as the Supabase project (Hobby allows one). Misalignment adds ~100ms per query. |
| Database | Supabase free Postgres, **pooled** connection (Supavisor, transaction mode) for the app |
| Migrations / dumps | **Direct** connection (port 5432) only |
| Images | Store BGG URL in M1; move thumbnails to Supabase Storage (1GB free) if hotlinking feels slow |
| Backups | "Export JSON" button + nightly `pg_dump` GitHub Action against the direct URL |

### Environment variables

```
DATABASE_URL        pooled, port 6543
DIRECT_URL          direct, port 5432 — migrations and dumps only
AUTH_SECRET         32+ random bytes
APP_URL             https://<project>.vercel.app   ← single source for every generated link
```

**Rule: build every join link, poll link and QR code from `APP_URL`, never from
`request.url`.** This one rule prevents the ugliest bug in this deployment.

### Trap 1 — preview URLs leak into QR codes

Every branch gets a preview deployment. If you open a poll there and hit share, the QR
encodes `project-git-branch-user.vercel.app`, which may not exist next week — and if
Deployment Protection is on, friends hit a 401 login wall instead of a ballot.

Mitigation: generate links only from `APP_URL`; test the share flow on production.
Preview URLs are for the developer, not for the group.

### Trap 2 — Supabase pauses after ~7 days idle

A quiet month means the first page load takes ~30 seconds to resume, while six people wait.
Mitigation: a daily Vercel cron hitting a cheap endpoint that touches the DB (also doubles as
an uptime check). Hobby cron allows one run per day — exactly enough.

Verify this actually prevents the pause rather than assuming. If it doesn't, ship a
"waking up the shelf" loading state; survivable, but embarrassing on a game night.

### Trap 3 — no long-running work

Hobby cron is daily, functions have a duration cap, and there is no queue. This is why BGG
is fetched **once, synchronously, on add**, then stored (§8). A "sync my whole BGG
collection" button is fine; a nightly re-sync of 200 games is not — that wants a VPS or a
paid tier.

### Session cookies on serverless

Invite token → httpOnly, `Secure`, `SameSite=Lax` cookie, 90-day expiry, server-side
revocable (no un-killable JWT). Rotate on reissue. Do **not** set a cookie `Domain` while on
`*.vercel.app` — leaving it host-only means a future domain move doesn't strand sessions.

### Game-night launch checklist

1. Open the poll on the **production** URL
2. Confirm the QR resolves to that same host
3. Vote once from cellular, not wifi — proves the network path
4. Hit the export button once, before you need it

### Future domain move

Buy the domain (~€10/yr) when you're ready, then keep the `*.vercel.app` host alive and
308-redirect it to the new one. Old group-chat links keep working; only `APP_URL` changes,
and everything generated from it follows automatically.

---

## 11. Testing

- Unit-testable pure functions: `score(games, constraints)`, `irv(ballots, vetoes)`,
  `fillSlots(budget, preset, shelf)`, `teachTime(game, attendees, plays)`,
  `slowdown(game, attendees)`. Write those first; they're where bugs live.
- `teachTime` gets a table test: the weight/length matrix above, one never-played attendee,
  one single-play attendee, 7 months stale, and the override field.
- `slowdown` gets the worked-example table as an assertion set, plus its +60% cap and the
  "measured bucket overrides formula" path.
- Seed fixtures: 20 games with deliberately messy data (null weight, unknown best-at,
  one vetoed, one lent out) and run the planner against them.
- The real test is **one actual game night**. Deploy before it, use it, write down every
  moment of friction. That session will change the roadmap more than any planning doc.

### Edge cases to cover explicitly

- Game with null weight / unknown best-at → degraded chip, never a confident claim
- Game never played by anyone → full teach, conservative plan
- One attendee has 1 play, rest have 20 → 60% teach, not full and not zero
- Everyone played it 7 months ago → refresh floor, not a full teach
- Game appears twice in a plan → duplicate penalty, never double-counted teach
- Late arrival misses the teach → flagged on the slot with a catch-up cost
- `teach_override_min` set → formula ignored entirely
- Three new players in a 2-hour game → +31% play time, plan still ends on time
- All-new table on a heavy game → slowdown capped at +60%, no absurd slot
- 4-player game where one attendee is rusty (≥180d) → half-novelty, not full teach
- Measured bucket `(game, players, new_players>0)` exists → median wins over the formula
- Someone arrives mid-game and misses the teach → slowdown still counts them, teach doesn't
- Headcount changes after the poll opens → mark plans stale, offer regenerate
- Fewer than 3 distinct plans possible → dedupe and show what exists
- All candidate plans share a vetoed game → say so, don't fail silently
- Every ballot exhausted → "host's call"
- 1-1-1 tie → deterministic tie-break, stated in the reveal
- Board game added but never played → shelf of shame, high staleness score
- Two copies of the same title → one shelf row, `owned ×2 · Alex, Priya`
- Game lent out → excluded unless the borrower is attending
- Misclick in the ballot → single undo restores rank, order and veto state exactly
- Ballot abandoned mid-way → stays a draft, never inflates "4 of 6 ballots in"
- Same user voting from two devices → last write wins on one ballot row, no duplicates

---

## 12. Open questions (decide during build, not blockers)

- Preset names: keep Compact / Varied / Party-ish?
- Can the host add a manual "locked" slot before generation ("build the night around Gloomhaven")?
- Are a winning plan's slots locked for the night? (Leaning: freely editable — the plan is a hypothesis.)
- One-off guest voting without an account? (Leaning: no; invite link takes 5 seconds and keeps stats honest.)
- Tonight's energy control: one slider vs three presets. (Leaning: three presets — faster to tap.)
- Winner auto-creates a play record, or host confirms? (Leaning: confirm.)

---

## 13. Build order

**M1**

1. Invite auth, group, members
2. Shelf: quick add + copies + lend/return + vetoes
3. Tonight: constraints → scored top 5 with reason chips
4. Poll: create, tap-to-rank with order strip + undo, server-side drafts, participation-only status
5. Close → IRV reveal with frozen snapshot, auto-close
6. Deploy: Vercel + Supabase, env vars, daily keepalive cron, export button (§10.1); use on a real game night

**M2**

7. Slot model, teach model (§4.2.1) + slowdown model (§4.2.2) + setup overlap,
   projected end time
8. Preset shapes + DP filler (budgeting on `est_minutes`)
9. `knows` set on the shelf for real teach and slowdown estimates
10. Editable slots: swap / lock / insert / remove, live warnings
11. Attendee windows (including late-arrival teach warnings)
12. Plan-level polls + slot diff in reveal

**M3**

13. Play logging with real durations and `teach_minutes`
14. Stats, shelf of shame, per-person records
15. Real durations and measured teach times feed the planner, replacing the formula
16. Taste profiles, optionally buying gaps
