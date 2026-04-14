# Manual Approval Gate

Manual Approval Gate is a Kubernetes Custom Resource Definition (CRD) controller. You can use it to add manual approval points in pipeline so that the pipeline wait at that point and waits for a manual approval before continuing execution

You can refer the ApprovalTask in the pipeline similar to how we refer Task today

```yaml
- name: wait
  taskRef:
    apiVersion: openshift-pipelines.org/v1alpha1
    kind: ApprovalTask
  params:
   - name: approvers
     value:
    	- foo # individual user
    	- bar 
    	- tekton 
        - group:tekton  # groupName
        - group:example  
   - name: numberOfApprovalsRequired
     value: 2
   - name: description
     value: Approval Task Rocks!!!
```

### Features 

* While referring the approvalTask in the pipelinerun following params need to be added 
  * approvers - The users who can approve/reject the approvalTask to unblock the pipeline 
  * numberOfApprovalsRequired - Numbers of approvals required to unblock the pipeline 
  * description - Description of approvalTask which users want to give
  
* Support for multiple users
  * Until and unless numberOfApprovalsRequired limit is not reached i.e approval task does not get approval from the users as approve, till then approvalState will be pending 
  * If any one approver rejects the approval task controller will mark the approvalState as rejected and then the pipelinerun will fail 
  * If a user approves for the first time and still approvalsRequired limit is not reached i.e. approvalState is in pending state then user can still change his input and mark the approval task as reject
  
* Support for approver groups
  * Define groups of users as approvers, using the group:<groupName> syntax.

* Individual & Group Approvers
  * Mix single users (alice, bob) and groups (group:dev-team, group:qa-team) in the approval list.

* Approval messages
  * Approvers can add a custom message when approving or rejecting.

* A webhook is configured while you install manual-approval-gate which will take care of all the checks which are required while the approver approves/rejects the approvalTask
* Users can add timeout to the approvalTask
* As of today once the timeout exceeds, approvalTask state is marked as rejected and correspondingly customrun and pipelinerun will be failed
* Users can add messages while approving/rejecting the approvalTask
* `tkn-approvaltask` CLI for managing approvaltasks

### Installation

*  On Kubernetes
    ```
    make apply
    ```
* On Openshift
   ```
   make TARGET=openshift apply
   ```
