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
        throw new Error('usage: check-app-and-cluster-status.js <kotsadm-ip>');
    }

    {
        const targetPage = page;
        await targetPage.setViewport({
            width: 1920,
            height: 934
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
                y: 15,
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
                x: 77,
                y: 21.2421875,
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
                x: 48,
                y: 7.7421875,
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
                x: 45,
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
                x: 35,
                y: 17.5078125,
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
                x: 27,
                y: 22.5078125,
              },
            });
    }
    {
        // CUSTOM: finding the element with the app state and extracting its content.
        let state = {app: "", cluster:""};
        process.stderr.write("waiting and fetching the application and cluster state\n");
        const targetPage = page;
        await targetPage.waitForSelector('#app > div > div.flex1.flex-column.u-overflow--auto.tw-relative > div > div > div > div.flex-column.flex1.u-position--relative.u-overflow--auto.u-padding--20 > div > div > div.flex.flex1.alignItems--center > div.u-marginLeft--20 > div > div:nth-child(1) > span:nth-child(6)');
        let elementContent = await targetPage.evaluate(() => {
            const element = document.querySelector('#app > div > div.flex1.flex-column.u-overflow--auto.tw-relative > div > div > div > div.flex-column.flex1.u-position--relative.u-overflow--auto.u-padding--20 > div > div > div.flex.flex1.alignItems--center > div.u-marginLeft--20 > div > div:nth-child(1) > span:nth-child(6)');
            return element ? element.textContent : null;
        });
        if (elementContent) {
            state.cluster = elementContent;
        }
        await targetPage.waitForSelector('#app > div > div.flex1.flex-column.u-overflow--auto.tw-relative > div > div > div > div.flex-column.flex1.u-position--relative.u-overflow--auto.u-padding--20 > div > div > div.flex.flex1.alignItems--center > div.u-marginLeft--20 > div > div:nth-child(1) > span:nth-child(2)');
        elementContent = await targetPage.evaluate(() => {
            const element = document.querySelector('#app > div > div.flex1.flex-column.u-overflow--auto.tw-relative > div > div > div > div.flex-column.flex1.u-position--relative.u-overflow--auto.u-padding--20 > div > div > div.flex.flex1.alignItems--center > div.u-marginLeft--20 > div > div:nth-child(1) > span:nth-child(2)');
            return element ? element.textContent : null;
        });
        if (elementContent) {
            state.app = elementContent;
        }
        console.log(JSON.stringify(state));
    }
    {
        process.stderr.write("checking for updates\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('div.u-paddingRight--15 div:nth-of-type(1) > span'),
            targetPage.locator('::-p-xpath(//*[@id=\\"app\\"]/div/div[2]/div/div/div/div[2]/div/div/div[2]/div[1]/div/div[1]/div/div/div[1]/span)'),
            targetPage.locator(':scope >>> div.u-paddingRight--15 div:nth-of-type(1) > span'),
            targetPage.locator('::-p-text(Check for update)')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 60.6953125,
                y: 9,
              },
            });
    }
    {
        process.stderr.write("deploying the new version\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(Deploy) >>>> ::-p-aria([role=\\"generic\\"])'),
            targetPage.locator('div.is-new > div.flex1 span'),
            targetPage.locator('::-p-xpath(//*[@id=\\"app\\"]/div/div[2]/div/div/div/div[2]/div/div/div[2]/div[1]/div/div[3]/div/div[1]/div[3]/div[2]/button/span)'),
            targetPage.locator(':scope >>> div.is-new > div.flex1 span')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 32.5078125,
                y: 5.90625,
              },
            });
    }
    {
        process.stderr.write("confirming that we want to deploy the new version\n");
        const targetPage = page;
        await puppeteer.Locator.race([
            targetPage.locator('::-p-aria(Yes, Deploy)'),
            targetPage.locator('button.u-marginLeft--10'),
            targetPage.locator('::-p-xpath(/html/body/div[8]/div/div/div/div/button[2])'),
            targetPage.locator(':scope >>> button.u-marginLeft--10'),
            targetPage.locator('::-p-text(Yes, Deploy)')
        ])
            .setTimeout(timeout)
            .click({
              offset: {
                x: 55.1484375,
                y: 19,
              },
            });
    }

    await browser.close();

})().catch(err => {
    console.error(err);
    process.exit(1);
});
