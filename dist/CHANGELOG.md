## Changelog
* a6eaaa71e7c34f9a0cf263c46eeb15fa8c140c17 feat(cli): add KeroAgile init onboarding command
* b27b9c1a5c9d9044a208562148d1cb255e2a8088 feat(cli): add sprint assign subcommand
* 19e77fa90e911682e4cdb56c46be8d56b0f0c9d4 feat(cli): use SuggestAssignee heuristic in task add when assignee not specified
* 3fcf4d1af176f97680e75ff3f028eeec80fd0c23 feat(domain): add AssignTaskToSprint service method and delegation helpers
* 0fec3c34256bebef1699957e56c29d8afba1afdb feat(domain): add SprintSummary type and extend Store interface with GetActiveSprint, ListSprintsWithCounts
* 2c65a77031812a49c1d7b97f3ca08cb202c12cb0 feat(domain): add SuggestAssignee heuristic for smart task routing
* 2559b58e7d2ecbed70ed375b3ce14e048290b26e feat(mcp): add assign_task_sprint tool
* ed798151b775073191a2e6569bf7fe4e3fe3fef3 feat(store): implement GetActiveSprint and ListSprintsWithCounts
* af2d7c671322a9324a572c7edbe47677a9aa7b97 feat(tui): add sprint message types
* fd333c156fed16193b9ea2c094b25b7cab39180e feat(tui): replace assignee text input with ‹ cycling › widget
* 97a0e084a20ea032f8efd535013f9626e0b7ae14 feat(tui/app): wire sprint selection, quick-assign keys, sprint form routing, and filtered task loading
* f85651b759e9c5f1c839a96de75bd8e6353012a4 feat(tui/board): add sprintHeader field for sprint filter display
* bcd18de7bc75da3bcda8f2501997c7077a025c33 feat(tui/forms): add SprintForm modal for sprint creation
* cf564207852cdc025dae9e23a6c5caded32c1b62 feat(tui/forms): add sprint field to task form with ID/name resolution
* 6804792cb6ec97c0a2e94f16a0988ee1f0331583 feat(tui/sidebar): add two-mode navigation for project list and sprint list
* 689f93f5d39ef4f37aee85cb0e80e2715fed9813 feat: board scrolling, MCP project/sprint tools, import slash command, demo GIFs
* 380ee0b83ffcc05ad5903c171af0c22309290f11 feat: merge sprint support — full sprint workflow across CLI, TUI, and MCP
* 4911ddceb6d50605d841b4c7fef4878c1b6e47f1 feat: smart assignee heuristic, assignee cycler, and KeroAgile init command
* d55d69594c45b38b1c5d540bed7dc84cb1400c1a fix(cli): shadow cfg variable, show real error on user creation, disallow trailing hyphen in ID
* b26cb1b11ed0889ffcd5a6a45c66d267d86680be fix(cli): show friendly message when init user already exists
* 3c085ab744ccacd3115d41f5ca02e42e998e27b0 fix(domain): add json tags to SprintSummary fields
* b4e622ab4d207651ef6d49eb8f0f5e916a98ce56 fix(tui): status-ordered flatIndex, drag ghost Y alignment, label chip rendering
* 447c3f824daebae681d874a2e147ef1b6b1b6703 fix(tui/app): quick-assign s/S keys reload filtered tasks without re-entering sprint list mode
* 2487c02fbe1636f726de55c062c92f936d622f60 fix(tui/app): wire SprintID from form SavedMsg into create and update task calls
* d9049a6814b9bebb585788a41046b7f947824f80 fix(tui/forms): consistent modulo wrap for left-arrow cycler, explicit assignee case in focusCurrent
* 176279f5b191ec3134e9471811676f8f2af19c5c fix: clear assigneeIdx on edit-form open, guard init prompts against EOF stdin, validate project ID
