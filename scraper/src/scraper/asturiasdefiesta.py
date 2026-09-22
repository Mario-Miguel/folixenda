

import asyncio
import datetime
import json
import logging
import re

import bs4

from src.services.database import import_events
from src.services.constants import EventCategory
from src.services import requests

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger(__name__)

BASE_URL = "https://www.asturiasdefiesta.es/wp-admin/admin-ajax.php"


async def get_pages_info(year: int, month: int) -> list[dict]:
    page_info = []
    page = 0

    logger.info("Fetching page %d — %s", page, BASE_URL)
    headers = {
        "content-type": "application/x-www-form-urlencoded; charset=UTF-8",
        "referer": "https://www.asturiasdefiesta.es/calendario-de-fiestas",
        "x-requested-with": "XMLHttpRequest"
    }
    status, response = await requests.request(requests.HTTPMethod.POST, url=BASE_URL, headers=headers, body=f"action=simcal_default_calendar_draw_grid&month={month}&year={year}&id=28061")
    if status == 200 and response:
        response_data = json.loads(response)
        soup = bs4.BeautifulSoup(response_data.get("data"), 'lxml')

        if soup:
            event_list:list = soup.find_all("li", class_="simcal-event")
            logger.info("Page %d: found %d events", page, len(event_list))

            for event in event_list:
                title = event.find("span", class_="simcal-event-title").text
                specific_url = event.find("a").attrs.get("href")

                raw_start_time = event.find("span", attrs={"itemprop": "startDate"}).attrs.get("content")
                raw_end_time = event.find("span", attrs={"itemprop": "endDate"}).attrs.get("content")                

                page_info_data = {
                    "url": specific_url,
                    "data":{
                        "title": title.strip() if title else None,
                        "start_time": raw_start_time,
                        "end_time": raw_end_time,
                        "venue": None,
                        "description": None,
                        "image_url": None
                    }
                }

                page_info.append(page_info_data)

    else:
        logger.warning("Page %d: unexpected status %d, stopping", page, status)

    logger.info("Page scraping done — %d events collected across %d pages", len(page_info), page)
    return page_info


async def get_specific_info(partial_info: dict, semaphore: asyncio.Semaphore, index: int, total: int):
    async with semaphore:
        url = partial_info.get("url")
        logger.info("[%d/%d] Fetching event detail — %s", index, total, url)
        status, response = await requests.request(requests.HTTPMethod.GET, url=url)
        
        if status == 200 and response:
            soup = bs4.BeautifulSoup(response, 'lxml')

            #address -> script con const mapaFiesta = L.map('map').setView

            full_event_info = partial_info.get("data")

            full_event_info["category"] = EventCategory.PARTIES.value
            
            return full_event_info


        logger.warning("[%d/%d] Failed to fetch detail (status %d), using partial data", index, total, status)
        return partial_info.get("data")


async def main():

    page_info: list[dict] = await get_pages_info()

    total = len(page_info)
    semaphore = asyncio.Semaphore(10)

    logger.info("Starting detail fetch for %d events (concurrency: 10)", total)
    async with asyncio.TaskGroup() as tg:
        tasks = [tg.create_task(get_specific_info(info, semaphore, i + 1, total)) for i, info in enumerate(page_info)]

    all_info: list[dict] = []

    for task in tasks:
        event_result = task.result()
        all_info.append(event_result)


    await import_events(all_info)



if __name__ == "__main__":
    asyncio.run(main())