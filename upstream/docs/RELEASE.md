# Release Process for Manual Approval Gate

* Step 1: Create a git tag and push 

    ```shell
    git tag <RELEASE_VERSION>     
    ```
  
    ```shell
    git push <remoteName> <latestTag>
    ```
  
* Step 2: A Github Action Workflow will be triggered and a new pre-release will be created

* Step 3: Inspect and refactor the release notes and publish the latest release 