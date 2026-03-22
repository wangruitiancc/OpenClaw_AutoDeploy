# OpenClaw AutoDeploy User Handbook

Welcome to OpenClaw AutoDeploy. This handbook explains everything you need to know to use the system as a tenant. No technical background is required — just follow the steps below.

---

## Table of Contents

1. [What is OpenClaw AutoDeploy?](#1-what-is-openclaw-autodeploy)
2. [Before You Start](#2-before-you-start)
3. [Install the CLI Tool](#3-install-the-cli-tool)
4. [Connect to the System](#4-connect-to-the-system)
5. [Your First Login](#5-your-first-login)
6. [Create Your Tenant Account](#6-create-your-tenant-account)
7. [Set Up Your API Key](#7-set-up-your-api-key)
8. [Configure Your Profile](#8-configure-your-profile)
9. [Deploy Your Container](#9-deploy-your-container)
10. [Check Deploy Status](#10-check-deploy-status)
11. [Access Your Deployed Service](#11-access-your-deployed-service)
12. [Stop and Start Your Service](#12-stop-and-start-your-service)
13. [View Your Deployment History](#13-view-your-deployment-history)
14. [Update Your Configuration](#14-update-your-configuration)
15. [Common Problems and Solutions](#15-common-problems-and-solutions)
16. [FAQ](#16-faq)

---

## 1. What is OpenClaw AutoDeploy?

OpenClaw AutoDeploy is a platform that automatically sets up and manages your own personal AI assistant container in the cloud. Think of it like renting a computer that's pre-configured to run your AI assistant — you don't need to worry about servers, networks, or technical setup.

**What you get:**
- A running AI assistant container accessible via a unique web address
- Secure storage for your API keys (so you don't have to manage them yourself)
- Automatic health monitoring — if something goes wrong, the system tries to fix it
- The ability to update your AI assistant without downtime

---

## 2. Before You Start

You will need:

- **A computer** (Windows, Mac, or Linux)
- **Internet connection**
- **An API key** from your AI provider (e.g., OpenAI, Anthropic, MiniMax) — this is what your AI assistant uses to think
- **Access credentials** for OpenClaw AutoDeploy (ask your administrator for your username/token)

---

## 3. Install the CLI Tool

The CLI tool is a text-based program called `openclawctl` that lets you control everything. Follow these steps to install it.

### Step 3.1: Download the Tool

Ask your administrator for the download link, or download from the releases page of the GitHub repository.

### Step 3.2: Install on Mac or Linux

Open your Terminal app and run:

```bash
# Make the file executable
chmod +x openclawctl

# Move it to a folder where your computer can find it
sudo mv openclawctl /usr/local/bin/
```

### Step 3.3: Install on Windows

1. Download the `.exe` file
2. Place it in a folder you can find easily (e.g., `C:\Users\YourName\OpenClaw\`)
3. Open Command Prompt and navigate to that folder:
   ```cmd
   cd C:\Users\YourName\OpenClaw
   ```

### Step 3.4: Verify Installation

After installing, run this command to confirm it works:

```bash
openclawctl version
```

You should see version information. If you see "command not found", try restarting your terminal app.

---

## 4. Connect to the System

Before you can do anything, you need to tell `openclawctl` where to find the OpenClaw server.

### Step 4.1: Get Your Server URL

Ask your administrator for the server URL. It will look something like:
```
https://api.openclaw.example.com
```

### Step 4.2: Configure the CLI

Run these commands to save your settings (replace the URL with your actual server URL):

```bash
openclawctl config init
openclawctl config set server https://api.openclaw.example.com
```

### Step 4.3: Save Your Token

Your administrator gave you a bearer token. Save it securely:

```bash
openclawctl config set token-file ~/.config/openclawctl/token
```

Then create the token file with your token:

```bash
# On Mac/Linux:
echo "YOUR_TOKEN_HERE" > ~/.config/openclawctl/token
chmod 600 ~/.config/openclawctl/token

# On Windows (Command Prompt):
# echo YOUR_TOKEN_HERE > %USERPROFILE%\.config\openclawctl\token
```

**Important:** Never share your token. It gives access to your account.

---

## 5. Your First Login

Verify that everything is set up correctly by checking if the system is reachable:

```bash
openclawctl health
```

You should see output indicating the system is healthy.

Also check if your credentials are working:

```bash
openclawctl ready
```

If both commands succeed, you're connected and authenticated.

---

## 6. Create Your Tenant Account

A "tenant" is your account on the OpenClaw system. You need to create one before you can deploy anything.

### Step 6.1: Get Your User ID

Ask your administrator for your external user ID. This is usually something like `user_10001`.

### Step 6.2: Create the Tenant

Run this command (replace the values with your own):

```bash
openclawctl tenant create \
  --external-user-id user_10001 \
  --slug my-name-001 \
  --display-name "My OpenClaw Account"
```

What these mean:
- `--external-user-id`: Your user ID from the administrator
- `--slug`: A short, unique nickname for your account (no spaces, use hyphens)
- `--display-name`: A friendly name shown in the interface

### Step 6.3: Verify It Was Created

List all tenants to confirm:

```bash
openclawctl tenant list
```

You should see your new tenant in the list.

---

## 7. Set Up Your API Key

Your AI assistant needs an API key to work. You store it securely in the system — it never appears in plain text after you enter it.

### Step 7.1: Get an API Key

Sign up with your AI provider and get an API key. For example:
- **OpenAI**: https://platform.openai.com/api-keys
- **Anthropic**: https://console.anthropic.com/
- **MiniMax**: https://platform.minimaxi.com/

Copy the key — it looks like a long string of letters and numbers (`sk-xxxx...`).

### Step 7.2: Store It Safely

Tell the system to save your API key (the system will read it from your environment variable):

```bash
export OPENAI_API_KEY="sk-xxxx_your_actual_key_here"
openclawctl secret set --tenant my-name-001 OPENAI_API_KEY --from-env OPENAI_API_KEY
```

Or save it from a file:

```bash
openclawctl secret set --tenant my-name-001 OPENAI_API_KEY --from-file ./my-api-key.txt
```

**Security note:** Your actual API key never appears in command output or logs.

### Step 7.3: Verify It Was Stored

List your secrets to confirm (you won't see the actual key, just the name):

```bash
openclawctl secret list --tenant my-name-001
```

---

## 8. Configure Your Profile

Your profile tells the system how to set up your AI assistant container.

### Step 8.1: Set Basic Profile

Run this command (ask your administrator for the correct values):

```bash
openclawctl profile set --tenant my-name-001 \
  --template tpl_standard \
  --tier standard \
  --route-key my-name-001 \
  --model-provider openai-compatible \
  --model-name gpt-4.1
```

What these mean:
- `--template`: Which template to use (ask your administrator)
- `--tier`: Resource tier (standard, large, etc.)
- `--route-key`: A unique key that becomes part of your service URL
- `--model-provider`: Your AI provider type
- `--model-name`: Which AI model to use

### Step 8.2: Validate Your Profile

Before deploying, make sure everything is configured correctly:

```bash
openclawctl profile validate --tenant my-name-001
```

If validation passes, you're ready to deploy.

---

## 9. Deploy Your Container

Now you're ready to start your AI assistant. Deploying creates your container and starts it running.

```bash
openclawctl deployment deploy --tenant my-name-001
```

The system will:
1. Create your container in the cloud
2. Start it up
3. Verify it's working

This usually takes 1-3 minutes.

### Wait for Deployment to Complete

If you want to wait and see the result:

```bash
openclawctl deployment deploy --tenant my-name-001 --wait --wait-timeout 180s
```

The command will keep checking until deployment succeeds or fails.

---

## 10. Check Deploy Status

### Step 10.1: View Current Instance

See what's currently running for your tenant:

```bash
openclawctl instance get --tenant my-name-001
```

### Step 10.2: View Pending Jobs

If a deployment is still in progress:

```bash
openclawctl job list --tenant my-name-001 --status pending
```

### Step 10.3: Watch a Specific Job

Watch the progress of a specific job:

```bash
openclawctl job watch --job ee18a31b-28a4-4937-9f42-6b78a0fda48f
```

(replace the job ID with the one you want to watch)

---

## 11. Access Your Deployed Service

Once deployed, your AI assistant is accessible at a unique web address:

```
http://my-name-001.localtest.me
```

(Replace `my-name-001` with your actual route key, and `localtest.me` with your actual domain)

Open this address in your web browser to use your AI assistant.

### If You Can't Access It

1. Wait 2-3 minutes — it takes time to start up
2. Check if your deployment succeeded (`openclawctl instance get --tenant my-name-001`)
3. See [Common Problems and Solutions](#15-common-problems-and-solutions)

---

## 12. Stop and Start Your Service

### Stop Your Service

Stop your container when you don't need it (saves resources):

```bash
openclawctl deployment stop --tenant my-name-001
```

Your data is preserved.

### Start Your Service Again

```bash
openclawctl deployment start --tenant my-name-001
```

### Restart Your Service

Restart if something seems wrong:

```bash
openclawctl deployment restart --tenant my-name-001
```

---

## 13. View Your Deployment History

See all past deployments:

```bash
openclawctl instance history --tenant my-name-001
```

This shows every time you've deployed, stopped, or updated — useful for troubleshooting.

---

## 14. Update Your Configuration

### Change Your API Key

If your AI provider key expired or changed:

```bash
# Set the new key
export NEW_API_KEY="sk-xxxx_new_key"
openclawctl secret set --tenant my-name-001 OPENAI_API_KEY --from-env NEW_API_KEY
```

### Redeploy with New Settings

After changing your profile:

```bash
openclawctl deployment redeploy --tenant my-name-001 --strategy replace
```

This replaces your current container with an updated one.

### Update Your Profile

Change AI model or other settings:

```bash
openclawctl profile set --tenant my-name-001 \
  --template tpl_standard \
  --tier standard \
  --route-key my-name-001 \
  --model-provider openai-compatible \
  --model-name gpt-4.1
```

Then redeploy.

---

## 15. Common Problems and Solutions

### Problem: "Command not found: openclawctl"

**Cause:** The tool isn't installed or not in your PATH.

**Solution:**
1. Find where you saved the file
2. Use the full path, e.g., `/usr/local/bin/openclawctl`
3. Or add that folder to your PATH

### Problem: "Authentication failed" or "token is invalid"

**Cause:** Your token is wrong or expired.

**Solution:**
1. Contact your administrator for a new token
2. Update your token file:
   ```bash
   echo "NEW_TOKEN" > ~/.config/openclawctl/token
   ```

### Problem: "Tenant not found"

**Cause:** The tenant doesn't exist or the slug is wrong.

**Solution:**
1. List all tenants to find the correct slug:
   ```bash
   openclawctl tenant list
   ```

### Problem: "Deployment failed" or container won't start

**Cause:** Configuration error or resource problem.

**Solution:**
1. Check job status:
   ```bash
   openclawctl job list --tenant my-name-001 --status pending
   ```
2. Validate your profile:
   ```bash
   openclawctl profile validate --tenant my-name-001
   ```
3. Check if your API key is valid
4. Contact your administrator

### Problem: "Secret not found"

**Cause:** You haven't set up your API key yet.

**Solution:**
```bash
openclawctl secret set --tenant my-name-001 OPENAI_API_KEY --from-env OPENAI_API_KEY
```

### Problem: Can't access my service at the URL

**Cause:** Service not ready or wrong URL.

**Solution:**
1. Check if the container is running:
   ```bash
   openclawctl instance get --tenant my-name-001
   ```
2. Wait 2-3 minutes for startup
3. Verify the URL with your administrator
4. Check health:
   ```bash
   curl http://127.0.0.1:8080/healthz
   ```

### Problem: Validation fails

**Cause:** Required configuration is missing.

**Solution:**
1. Check what's missing — usually a template, API key, or route key
2. Make sure all required fields are set
3. Ask your administrator for correct values

---

## 16. FAQ

### Q: Is my API key safe?
**A:** Yes. Your API key is encrypted and stored securely. It never appears in logs or command output.

### Q: What happens if my AI provider has an outage?
**A:** Your AI assistant won't work until the provider is back. The system will automatically retry when it's restored.

### Q: Can I use multiple AI providers?
**A:** Yes. Ask your administrator how to configure multiple providers.

### Q: What if I need more resources?
**A:** Contact your administrator to upgrade your resource tier.

### Q: Can I destroy my deployment and start over?
**A:** Yes:
```bash
openclawctl deployment destroy --tenant my-name-001 --yes
```
This removes your container. Your profile and secrets are preserved.

### Q: How do I know if my container is healthy?
**A:** Check the instance:
```bash
openclawctl instance get --tenant my-name-001
```

### Q: What's the difference between stop and destroy?
**A:** **Stop** pauses your container and preserves data. **Destroy** removes the container entirely.

### Q: Who do I contact for help?
**A:** Contact your system administrator. If they can't help, they will escalate to the OpenClaw team.
