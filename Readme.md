# PickemGoLiveStats

This is a simple application for processing (semi)-live statistics from currently live MLB games. The program persists checking until all games *of the day* are completed. 

## Requirements

0. Setup a .env with a Postgres & Redis URL (with `redis://` or `postgresql://`) **with the username and password included in the url,** example is `postgresql://postgres:password@host:port/database` for Postgres, and `redis://redis:password@host:post`.
1. Setup a Postgres database by running an automigrate script from the [PickemAPI](https://github.com/specificlanguage/PickemAPI), which this ties into.
2. Setup a Redis cache, which this will be writing into.
3. Build the app through `go build *.go -o [dest path]`.
4. Run the application through the binary that was just built. (`./[dest path]`)

## Why?

The Pick'em project allows users to select games that are currently live. This particular app allows us to process the statistics within a reasonable amount of time, about every pitch or so (if we want to in the future.)
Go's simple concurrency model allows us to do this quite easily, and lets us update games in real time, and deliver the answers once the final score arrives.

We can't necessarily do this in the Python API as it would require a little too much overhead. Doing this in a separate, cron job application would allow this to take little memory but provide a lot of data for us to use, especially in the future.

## How can I try this out without setting all of this up?

This is part of a suite of repositories that make up the entire Baseball Pick'em app. See the [API (Python/FastAPI)](https://github.com/specificlanguage/PickemAPI) and [frontend (React/Vite)](https://github.com/specificlanguage/pickem-react) in their respective items.
