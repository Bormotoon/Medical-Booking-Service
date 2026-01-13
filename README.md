# Medical Booking Service

Monorepo containing bronivik_jr and bronivik_crm.

## Updating subprojects (git subtree)

- If remotes are not set yet, add them:

	```bash
	git remote add bronivik_jr https://github.com/Bormotoon/bronivik_jr.git
	git remote add bronivik_crm https://github.com/Bormotoon/bronivik_crm.git
	```

- Pull latest changes into the `bronivik_jr/` subtree (source branch: `master`):

	```bash
	git fetch bronivik_jr master
	git subtree pull --prefix=bronivik_jr bronivik_jr master
	```

- Pull latest changes into the `bronivik_crm/` subtree (source branch: `main`):

	```bash
	git fetch bronivik_crm main
	git subtree pull --prefix=bronivik_crm bronivik_crm main
	```

- To push local changes made inside a subtree back to its upstream:

	```bash
	git subtree push --prefix=bronivik_jr bronivik_jr master
	git subtree push --prefix=bronivik_crm bronivik_crm main
	```

- Notes:
	- Use `--squash` with `git subtree pull` if you prefer a single merge commit instead of preserving all upstream commits.
	- Ensure you run these commands from the repository root where the `bronivik_jr/` and `bronivik_crm/` folders live.
