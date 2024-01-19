#!/usr/bin/env node

/*
 * this script has been generated with chrome recorder and then pasted here.
 * some parts were manually changed, these are flagged with a CUSTOM comment.
 * all logging has also been manually added (process.stderr.write() calls).
 * this script is meant to be run as an argument to the `puppeteer.sh` script.
 */

const puppeteer = require('puppeteer'); // v20.7.4 or later

(async () => {
    const browser = await puppeteer.launch(
        {
            headless: 'new',
            // CUSTOM: added the following line to fix the "No usable sandbox!" error.
            args: ['--no-sandbox', '--disable-setuid-sandbox']
        }
    );
    const page = await browser.newPage();
    const timeout = 5000;
    page.setDefaultTimeout(timeout);
    const args = process.argv.slice(2);
    if (args.length !== 1) {
        throw new Error('usage: generate-controller-join-token.js <kotsadm-ip>');
    }

    {
        const targetPage = page;
        await targetPage.setViewport({
            width: 1512,
            height: 761
        })
    }
    {
        process.stderr.write("opening a new tab\n");
        const targetPage = page;
        const promises = [];
        const startWaitingForEvents = () => {
            promises.push(targetPage.waitForNavigation());
        }
        startWaitingForEvents();
        await targetPage.goto('chrome://new-tab-page/');
        await Promise.all(promises);
    }
    {
        process.stderr.write("acessing kotsadm on port 30000\n");
        const targetPage = page;
        const promises = [];
        const startWaitingForEvents = () => {
            promises.push(targetPage.waitForNavigation());
        }
        startWaitingForEvents();
        // CUSTOM: using the command line argument.
        await targetPage.goto(`http://${args[0]}:30000/`);
        await Promise.all(promises);
    }
    {
        process.stderr.write("waiting and clickin on the 'Continue to Setup' button\n");
        const targetPage = page;
        const promises = [];
        const startWaitingForEvents = () => {
            promises.push(targetPage.waitForNavigation());
        }
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(Continue to Setup)'),
            targetPage.locator('button'),
            targetPage.locator('::-p-xpath(/html/body/div/div/div[2]/div[1]/div[4]/button)'),
            targetPage.locator(':scope >>> button'),
            targetPage.locator('::-p-text(Continue to Setup)')
        ])
            .setTimeout(timeout)
            .on('action', () => startWaitingForEvents())
            .click({
              offset: {
                x: 44,
                y: 7,
              },
            });
        await Promise.all(promises);
    }
    {
        process.stderr.write("waiting and clicking on 'Advanced' to move on with the certificate\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(Advanced)'),
            targetPage.locator('#details-button'),
            targetPage.locator('::-p-xpath(//*[@id=\\"details-button\\"])'),
            targetPage.locator(':scope >>> #details-button'),
            targetPage.locator('::-p-text(Advanced)')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 31,
                y: 18.4609375,
              },
            });
    }
    {
        process.stderr.write("waiting and clicking on 'Proceed' to move on with the certificate\n");
        const targetPage = page;
        // CUSTOM: using command line argument.
        await puppeteer.Locator.race([
            targetPage.locator(`::-p-aria(Proceed to ${args[0]} \\(unsafe\\))`),
            targetPage.locator('#proceed-link'),
            targetPage.locator('::-p-xpath(//*[@id=\\"proceed-link\\"])'),
            targetPage.locator(':scope >>> #proceed-link'),
            targetPage.locator(`::-p-text(Proceed to ${args[0]})`)
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 44,
                y: 6.9609375,
              },
            });
    }
    {
        process.stderr.write("going to the /tls endpoint\n");
        const targetPage = page;
        const promises = [];
        const startWaitingForEvents = () => {
            promises.push(targetPage.waitForNavigation());
        }
        startWaitingForEvents();
        // CUSTOM: using the command line argument.
        await targetPage.goto(`https://${args[0]}:30000/tls`);
        await Promise.all(promises);
    }
    {
        process.stderr.write("waiting and clicking on 'Continue'\n");
        const targetPage = page;
        const promises = [];
        const startWaitingForEvents = () => {
            promises.push(targetPage.waitForNavigation());
        }
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(Continue) >>>> ::-p-aria([role=\\"generic\\"])'),
            targetPage.locator('div.button-wrapper span.self-signed-visible'),
            targetPage.locator('::-p-xpath(//*[@id=\\"upload-form\\"]/div[6]/button/span[1])'),
            targetPage.locator(':scope >>> div.button-wrapper span.self-signed-visible'),
            targetPage.locator('::-p-text(Continue)')
        ])
            .setTimeout(timeout)
            .on('action', () => startWaitingForEvents())
            .click({
              offset: {
                x: 39,
                y: 8,
              },
            });
        await Promise.all(promises);
    }
    {
        process.stderr.write("waiting and clicking in the password field\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(password)'),
            targetPage.locator('input'),
            targetPage.locator('::-p-xpath(//*[@id=\\"app\\"]/div/div[2]/div/div/div/div[2]/div/div/div[1]/input)'),
            targetPage.locator(':scope >>> input')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 27,
                y: 21.0078125,
              },
            });
    }
    {
        process.stderr.write("typing the password\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(password)'),
            targetPage.locator('input'),
            targetPage.locator('::-p-xpath(//*[@id=\\"app\\"]/div/div[2]/div/div/div/div[2]/div/div/div[1]/input)'),
            targetPage.locator(':scope >>> input')
        ])
            .setTimeout(timeout)
            .fill('password');
    }
    {
        process.stderr.write("clicking in the Log in button\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(Log in)'),
            targetPage.locator('button'),
            targetPage.locator('::-p-xpath(//*[@id=\\"app\\"]/div/div[2]/div/div/div/div[2]/div/div/div[2]/button)'),
            targetPage.locator(':scope >>> button')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 30,
                y: 14.0078125,
              },
            });
    }
    {
        process.stderr.write("waiting and clicking in the Cluster Management tab\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('div:nth-of-type(3) > span'),
            targetPage.locator('::-p-xpath(//*[@id=\\"app\\"]/div/div[1]/div[1]/div[2]/div[3]/span)'),
            targetPage.locator(':scope >>> div:nth-of-type(3) > span'),
            targetPage.locator('::-p-text(Cluster Management)')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 87.734375,
                y: 26,
              },
            });
    }
    {
        process.stderr.write("waiting and clicking in the Add node button\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(Add node)'),
            targetPage.locator('div.tw-flex > button'),
            targetPage.locator('::-p-xpath(//*[@id=\\"app\\"]/div/div[2]/div/div/div[1]/button)'),
            targetPage.locator(':scope >>> div.tw-flex > button'),
            targetPage.locator('::-p-text(Add node)')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 24.328125,
                y: 17,
              },
            });
    }
    {
        process.stderr.write("waiting and clicking in the controller role\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('div:nth-of-type(1) > label'),
            targetPage.locator('::-p-xpath(/html/body/div[5]/div/div/div/div[2]/div[1]/label)'),
            targetPage.locator(':scope >>> div:nth-of-type(1) > label')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 110,
                y: 27.5,
              },
            });
    }
    {
        // CUSTOM: finding the element that contains the node join command.
        process.stderr.write("waiting and fetching the node join command\n");
        const targetPage = page;
        await targetPage.waitForSelector('.react-prism.language-bash');
        let elementContent = await targetPage.evaluate(() => {
            const element = document.querySelector('.react-prism.language-bash');
            return element ? element.textContent : null;
        });
        if (!elementContent) {
            throw new Error("Could not find the node join command");
        }
        console.log(elementContent);
    }

    await browser.close();

})().catch(err => {
    console.error(err);
    process.exit(1);
});
