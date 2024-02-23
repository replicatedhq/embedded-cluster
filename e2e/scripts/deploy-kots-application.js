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
    const timeout = 30000;
    page.setDefaultTimeout(timeout);

    const args = process.argv.slice(2);
    if (args.length !== 1) {
        throw new Error('usage: deploy-kots-application.js <kotsadm-ip>');
    }

    {
        const targetPage = page;
        await targetPage.setViewport({
            width: 1920,
            height: 934
        })
    }
    {
        process.stderr.write("opening a new page\n");
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
        process.stderr.write("opening kots page\n");
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
        process.stderr.write("waiting and clicking on the 'Continue to Setup' button\n");
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
                x: 52,
                y: 13,
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
                x: 69,
                y: 23.2421875,
              },
            });
    }
    {
        process.stderr.write("waiting and clicking on 'Proceed' to move on with the certificate\n");
        const targetPage = page;
        // CUSTOM: using command line argument.
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(Proceed to 192.168.86.6 \\(unsafe\\))'),
            targetPage.locator('#proceed-link'),
            targetPage.locator('::-p-xpath(//*[@id=\\"proceed-link\\"])'),
            targetPage.locator(':scope >>> #proceed-link'),
            targetPage.locator(`::-p-text(Proceed to ${args[0]})`)
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 75,
                y: 9.7421875,
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
        const targetPage = page;
        const promises = [];
        const startWaitingForEvents = () => {
            promises.push(targetPage.waitForNavigation());
        }
        process.stderr.write("waiting and clicking on 'Continue'\n");
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(Continue)'),
            targetPage.locator('button'),
            targetPage.locator('::-p-xpath(//*[@id=\\"upload-form\\"]/div[6]/button)'),
            targetPage.locator(':scope >>> button'),
            targetPage.locator('::-p-text(Continue\n   )')
        ])
            .setTimeout(timeout)
            .on('action', () => startWaitingForEvents())
            .click({
              offset: {
                x: 43,
                y: 6,
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
                x: 32,
                y: 24.5078125,
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
                y: 21.5078125,
              },
            });
    }
    {
        process.stderr.write("clicking on Continue in the cluster management page\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(Continue)'),
            targetPage.locator('div.flex1 > div > div > button'),
            targetPage.locator('::-p-xpath(//*[@id=\\"app\\"]/div/div[2]/div/div/button)'),
            targetPage.locator(':scope >>> div.flex1 > div > div > button'),
            targetPage.locator('::-p-text(Continue)')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 45.9921875,
                y: 6.5859375,
              },
            });
    }
    {
        process.stderr.write("clicking on the configuration field so we can enter the application configuration\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('input'),
            targetPage.locator('::-p-xpath(//*[@id=\\"hostname-group\\"]/div[2]/div/input)'),
            targetPage.locator(':scope >>> input')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 350.4296875,
                y: 24.8046875,
              },
            });
    }
    {
        process.stderr.write("typing 'abc' as the configuration value\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('input'),
            targetPage.locator('::-p-xpath(//*[@id=\\"hostname-group\\"]/div[2]/div/input)'),
            targetPage.locator(':scope >>> input')
        ])
            .setTimeout(timeout)
            .fill('abc');
    }
    {
        process.stderr.write("clicking on Continue after filling in the configuration\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(Continue)'),
            targetPage.locator('button'),
            targetPage.locator('::-p-xpath(//*[@id=\\"app\\"]/div/div[2]/div/div/div/div[2]/div/div[2]/div/button)'),
            targetPage.locator(':scope >>> button'),
            targetPage.locator('::-p-text(Continue)')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 44.4296875,
                y: 11.8046875,
              },
            });
    }

    await browser.close();

})().catch(err => {
    console.error(err);
    process.exit(1);
});
