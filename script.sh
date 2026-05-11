
#!/bin/bash
# Usage: ./create-ocp-user.sh <username> <password>
# Example: ./create-ocp-user.sh tekton-user1 mypassword123

set -e

if [ $# -ne 2 ]; then
  echo "Usage: $0 <username> <password>"
  exit 1
fi

USER=$1
PASS=$2
HTPASSWD_FILE=users.htpasswd
SECRET_NAME=htpass-secret
NAMESPACE=openshift-config

echo "[INFO] Creating/Updating htpasswd file..."
if oc get secret $SECRET_NAME -n $NAMESPACE >/dev/null 2>&1; then
  # Extract existing htpasswd file
  oc extract secret/$SECRET_NAME -n $NAMESPACE --to=. --confirm
  htpasswd -B -b $HTPASSWD_FILE $USER $PASS
else
  # Create new htpasswd file
  htpasswd -c -B -b $HTPASSWD_FILE $USER $PASS
fi

echo "[INFO] Updating htpasswd secret..."
oc create secret generic $SECRET_NAME \
  --from-file=htpasswd=$HTPASSWD_FILE \
  -n $NAMESPACE \
  --dry-run=client -o yaml | oc apply -f -

echo "[INFO] Ensuring OAuth is configured..."
# Check if HTPasswd provider already exists, if not, add it
if ! oc get oauth cluster -o yaml | grep -q "htpasswd_provider"; then
  oc patch oauth cluster --type='json' -p='[{"op": "add", "path": "/spec/identityProviders/-", "value":{"name":"htpasswd_provider","mappingMethod":"claim","type":"HTPasswd","htpasswd":{"fileData":{"name":"htpass-secret"}}}}]'
else
  echo "[INFO] OAuth provider already configured. Skipping patch."
fi

echo "[INFO] User $USER created/updated successfully!"
echo "👉 Try logging in with:"
echo "   oc login https://api.<cluster-domain>:6443 -u $USER -p $PASS"
