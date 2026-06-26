

import asyncio
import datetime
import logging
import re

import bs4

from src.services.database import import_events
from src.services.constants import EventCategory
from src.services import requests

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger(__name__)

BASE_URL = "https://feteas.org"


async def get_pages_info() -> list[dict]:
    page_info = []
    page = 0

    while True:
        page += 1

        url = f"{BASE_URL}/agenda/lista/p%C3%A1gina/{page}/"

        logger.info("Fetching page %d — %s", page, url)
        status, response = await requests.request(requests.HTTPMethod.GET, url=url)
        if status == 200 and response:
            soup = bs4.BeautifulSoup(response, 'lxml')

            if error_message := soup.find("div", class_="tribe-events-c-messages__message tribe-events-c-messages__message--notice"):
                if "no se ha encontrado ningún resultado" in error_message.text.lower():
                    break

            if events_container := soup.find("ul", class_="tribe-events-calendar-list"):
                event_list:list = events_container.find_all("li", class_="tribe-common-g-row")
                logger.info("Page %d: found %d events", page, len(event_list))

                for event in event_list:
                    date = event.find("time").attrs.get("datetime")
                    title = event.find("h4").text
                    specific_url = event.find("a").attrs.get("href")
                    venue = event.find("span", class_="tribe-events-calendar-list__event-venue-title")
                    description = event.find("p")
                    image_url = event.find("img", class_="tribe-events-calendar-list__event-featured-image")

                    page_info_data = {
                        "url": specific_url,
                        "data":{
                            "title": title.strip() if title else None,
                            "date": date,
                            "venue": venue.text.strip() if venue else None,
                            "description": description.text.strip() if description else None,
                            "image_url": image_url.attrs.get("src") if image_url else None
                        }
                    }

                    if raw_datetime := event.find("time", class_="tribe-events-calendar-list__event-datetime"):
                        datetime_text = raw_datetime.text.strip()
                        if match := re.match(r".*?\| (?P<start_time>\d+:\d+) - .*?\| (?P<end_time>\d+:\d+)", datetime_text):
                            start_time = match.group("start_time")
                            end_time = match.group("end_time")
                            page_info_data["data"]["start_time"] = start_time
                            page_info_data["data"]["end_time"] = end_time
                        elif match:= re.match(r".*?\| (?P<start_time>\d+:\d+) - (?P<end_time>\d+:\d+)", datetime_text):
                            starts_at = match.group("start_time")
                            end_time = match.group("end_time")
                            page_info_data["data"]["start_time"] = starts_at
                            page_info_data["data"]["end_time"] = end_time


                    page_info.append(page_info_data)
            else:
                break
        else:
            logger.warning("Page %d: unexpected status %d, stopping", page, status)
            break

    logger.info("Page scraping done — %d events collected across %d pages", len(page_info), page)
    return page_info


async def get_specific_info(partial_info: dict, semaphore: asyncio.Semaphore, index: int, total: int):
    async with semaphore:
        url = partial_info.get("url")
        logger.info("[%d/%d] Fetching event detail — %s", index, total, url)
        status, response = await requests.request(requests.HTTPMethod.GET, url=url)
        
        if status == 200 and response:
            soup = bs4.BeautifulSoup(response, 'lxml')

            full_event_info = partial_info.get("data")

            raw_description = soup.find("div", class_="tribe-events-single-event-description tribe-events-content")

            full_event_info["description"] = raw_description.text.strip() if raw_description else full_event_info["description"]

            if artist_div := soup.find("div", class_="tribe-events-meta-group tribe-events-meta-group-other"):
                artist = artist_div.find("dd")
                full_event_info["artist"] = artist.text.strip() if artist else None
            
            if venue_div := soup.find("div", class_="tribe-events-meta-group tribe-events-meta-group-venue"):
                venue = venue_div.find("dd")
                full_event_info["venue"] = venue.text.strip() if venue else full_event_info["venue"]

                # Address -> consulta a google maps, hacer a posteriori
            full_event_info["category"] = EventCategory.THEATER.value
            
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