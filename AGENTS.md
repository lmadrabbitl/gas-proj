# Repo Instructions

## Project Context

- This project is a personal expense tracker app.
- Frontend lives in `ui/` and uses Angular.
- Backend lives in `code/` and uses Go.
- The app is currently written in `pt-BR`.
- In the future, the app should also support `en-US`.
- UI messages are centralized in separate files to support localization.
- Each user has a `user_config` row in the database for settings and language.

## Working Preferences

- Preserve the current localization structure when changing UI text or user-facing messages.

## Post-change verification

- After UI-only changes, run `./buildImageAndRunCompose.sh ui`
- After backend-only changes, run `./buildImageAndRunCompose.sh code`
- After changes that touch both UI and backend, run `./buildImageAndRunCompose.sh`
