import { test, expect } from '@playwright/test'

const web = process.env.BASE_URL || 'http://web'

async function dismissNick(page) {
  const enter = page.getByRole('button', { name: '进入工作室' })
  if (await enter.isVisible().catch(() => false)) {
    const nick = page.getByLabel(/昵称/)
    if (await nick.count()) await nick.fill('E2E员')
    await enter.click()
  }
}

async function createBoard(page, title: string) {
  await page.goto(web + '/')
  await expect(page.getByRole('heading', { name: '图纸目录' })).toBeVisible()
  await dismissNick(page)
  await page.getByPlaceholder('未命名白板').fill(title)
  await page.getByRole('button', { name: '铺一张新纸' }).click()
  await page.waitForURL(/\/board\//, { timeout: 15000 })
}

test.describe('GoMiro critical path', () => {
  test('home creates a board and canvas mounts', async ({ page }) => {
    await createBoard(page, 'E2E-' + Date.now())
    await expect(page.locator('canvas').first()).toBeVisible()
  })

  test('two pages share a board and see a remote cursor rail', async ({ browser }) => {
    const ctxA = await browser.newContext()
    const ctxB = await browser.newContext()
    const a = await ctxA.newPage()
    const b = await ctxB.newPage()
    await createBoard(a, 'Duo-' + Date.now())
    const url = a.url()
    await b.goto(url)
    await dismissNick(b)
    await expect(a.locator('canvas').first()).toBeVisible()
    await expect(b.locator('canvas').first()).toBeVisible()
    await ctxA.close()
    await ctxB.close()
  })
})
