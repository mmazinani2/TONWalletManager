const express = require('express');
const bodyParser = require('body-parser');
const { exec } = require('child_process');
const fs = require('fs');
const util = require('util');
const execPromise = util.promisify(exec);

const app = express();
const port = 3000;

let isLocked = false;

app.use(bodyParser.json());
app.use(bodyParser.urlencoded({ extended: true }));

// Serve static files
app.use(express.static('public'));

app.get('/', (req, res) => {
  res.sendFile(__dirname + '/public/index.html');
});

async function handleRequest(res, command, input, parser) {
  if (isLocked) {
    return res.status(429).send('Another operation is in progress. Please try again later.');
  }

  isLocked = true;
  try {
    const { stdout, stderr } = await execPromise(command, { input });
    if (stderr) {
      console.error(`exec stderr: ${stderr}`);
      return res.status(500).send(stderr);
    }
    const result = parser(stdout.toString());
    if (result.error) {
      return res.status(500).send(result.error);
    }
    res.json(result.data);
  } catch (error) {
    console.error(`exec error: ${error}`);
    res.status(500).send(error.message);
  } finally {
    isLocked = false;
  }
}

// Endpoint to create a new wallet
app.post('/create-wallet', async (req, res) => {
  await handleRequest(res, 'go run main.go', '1\n5\n', (output) => {
    const seedPhraseMatch = output.match(/Seed: (.*)/);
    const walletAddressMatch = output.match(/Wallet Address: (.*)/);

    if (!seedPhraseMatch || !walletAddressMatch) {
      return { error: 'Error parsing Go application output for wallet creation.' };
    }

    return { data: { seedPhrase: seedPhraseMatch[1], walletAddress: walletAddressMatch[1] } };
  });
});

// Endpoint to check wallet balance
app.post('/check-balance', async (req, res) => {
  await handleRequest(res, 'go run main.go', '2\n5\n', (output) => {
    const balanceMatch = output.match(/Balance: (.*)/);
    const walletAddressMatch = output.match(/Wallet address: (.*)/);

    if (!balanceMatch || !walletAddressMatch) {
      return { error: 'Error parsing Go application output for balance check.' };
    }

    return { data: { balance: balanceMatch[1], walletAddress: walletAddressMatch[1] } };
  });
});

// Endpoint to send transactions
app.post('/send-transactions', async (req, res) => {
    if (isLocked) {
        return res.status(429).send('Another operation is in progress. Please try again later.');
    }
    isLocked = true;
    try {
        const { receivers, comment } = req.body;

        // Update receivers.txt
        let receiverContent = '';
        for (const receiver of receivers) {
            receiverContent += `${receiver.address} ${receiver.amount}\n`;
        }
        fs.writeFileSync('receivers.txt', receiverContent);

        // Update config.txt
        const config = fs.readFileSync('config.txt', 'utf8');
        const newConfig = config.replace(/comment=.*/, `comment=${comment}`);
        fs.writeFileSync('config.txt', newConfig);

        const { stdout, stderr } = await execPromise('go run main.go', { input: '4\n5\n' });
        if (stderr) {
            console.error(`exec stderr: ${stderr}`);
            return res.status(500).send(stderr);
        }
        const transactionHashMatch = stdout.toString().match(/Transaction sent, hash: (.*)/);
        if (!transactionHashMatch) {
            return res.status(500).send('Error parsing Go application output for sending transactions.');
        }
        res.json({ transactionHash: transactionHashMatch[1] });
    } catch (error) {
        console.error(`exec error: ${error}`);
        res.status(500).send(error.message);
    } finally {
        isLocked = false;
    }
});

app.listen(port, () => {
  console.log(`Server listening at http://localhost:${port}`);
});
