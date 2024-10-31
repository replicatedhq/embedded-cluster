## Running Tests Against the Development Environment

1. Install deps on your Mac

   ```bash
   npm ci
   npx playwright install --with-deps
   code --install-extension ms-playwright.playwright
   ```

1. Build the release and run the development environment

   ```bash
   make initial-release
   make create-node0
   output/bin/embedded-cluster install --license local-dev/license.yaml
   ```

1. Create a nodeport service directly to kotsadm

   ```bash
   cat <<EOF | k0s kubectl apply -f -
   apiVersion: v1
   kind: Service
   metadata:
     name: kotsadm-nodeport
     namespace: kotsadm
     labels:
       replicated.com/disaster-recovery: infra
       replicated.com/disaster-recovery-chart: admin-console
   spec:
     type: NodePort
     ports:
     - port: 30003
       targetPort: 3000
       nodePort: 30003
     selector:
       app: kotsadm
   EOF
   ```

1. Configure the base URL (default is `http://localhost:30000`) on your Mac

   ```bash
   export BASE_URL=http://localhost:30003
   ```

1. Run the test on your Mac

   ```bash
   npx playwright test --ui deploy-app
   ```

   Don't forget to press the play button in the browser to run the test.
