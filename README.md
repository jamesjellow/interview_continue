# Fast Package Manager (fpm)

Fast Package Manager (fpm) is a tool to create packages.

![Demo](https://media2.giphy.com/media/v1.Y2lkPTc5MGI3NjExZTI3M3lxY2FpamJrNGMyZTY2NXVtcXh2dTkweDBwdnBnY2hiYTFqaCZlcD12MV9pbnRlcm5hbF9naWZfYnlfaWQmY3Q9Zw/5J0tFwHsOFoJ7Dq0pw/giphy.gif)

## Usage

```bash
$ fpm add <packageName@version> # Add a dependency (pass -D for dev dependency)
```

```bash
$ fpm install # Install all dependencies from package.json
```

## Documentation

1. `fpm add <package_name>` - Adds the dependency to the “dependencies” object in package.json
   - This will take a single argument, which is the name of the package
   - The package might include a version, delimited by “@” like “is-thirteen@0.1.13”, which it should parse
   - It should write to an _existing_ (you can create it manually or with `npm init`) package.json to add `"is-thirteen": "0.1.13"` to the `dependencies` object
2. `install` - Downloads all of the packages that are specified in package.json, as well as package that are dependencies of these
   - Should read the `dependencies` object of the package.json
   - Assume that the node_modules folder is currently empty, rather than trying to determine what exists or not
   - Determine all dependencies of dependencies
   - Download each to the node_modules folder

## Installation

```bash
$ git clone https://github.com/jamesjellow/interview_continue
$ cd interview_continue
```

```bash
$ go build .
$ go install
```

Now you can use `fpm` cli tool!

## Design Decisions

1. Why Go?

   - It's fast
   - It's easy to read
   - It's easy to write
   - It's a compiled which means fast execution through native binaries
   - It's got great dependency management
   - It has modern high level syntax features
   - It has low level access to memory addresses and pointer
   - It has great support for concurrency
   - It has a great standard library
   - It can compile to different platforms (Mac, Windows, Linux)

2. Why use a dependency graph?

   - Check for circular dependencies
   - Avoid unnecessary and redudant download

3. What is the call flow?

   - The call flow is
     `main -> controller -> util -> pkgmanager`
   - The is similar to the pattern
     `entry -> router -> main logic -> repository`

4. Why hardcode package.json and node_modules?
   - This cli tool assumes that you have a `package.json` and file and `node_modules/` in your `cwd`

## FAQ

- **Dependency conflict resolution: what happens if two dependencies require different versions of another dependency?**
  - The tool will resolve the conflict by taking the highest version of the dependency.
- **Lock file: How can you make sure that installs are deterministic?**
  - Instead of using a package-lock file, we are using a graph to check for circular dependencies.
- **Caching: It’s a waste of storage and time to be redownloading a package that you’ve already downloaded for another project. How can you save something globally to avoid extra downloads? Are there different levels of efficiency you could achieve?**

  - The cli tool checks if the package exists in the `node_modules/` folder and if so skips the installation. Additionally, the tool uses the dependency graph to check for verticies that already exist.

        Caching levels:

        - Global cache: CDN/Redis to cache redundant downloads/requests
        - Network level cache: Cache network requests (automatic)
        - Local Cache: Check if exists in the node_modules/ folder or utilize a /cache folder

- **Validation: How can you verify that an installation of a package is correct?**
  - The tool validates the checksum upon download
- **Circular dependencies: What happens if there is a dependency graph like A → B → C → A?**
  - The tool will detect and skip circular dependencies using a graph to prevent cycles.
- **Fun animations?**
  - Animations are being used from here github.com/briandowns/spinner

## Todos

- [ ] Add more tests
- [ ] Add more options for the user (e.g. `fpm rm`)
- [ ] Add more error handling (e.g. when a package is not found, etc.)
- [ ] Add more logging (e.g. verbose, debug, etc.)
- [ ] Add thread safety and concurrency support
- [ ] Fix issue of packages being missed and not installed
