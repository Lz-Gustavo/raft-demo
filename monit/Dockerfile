FROM python:3.7-slim

RUN mkdir /monit
WORKDIR /monit

COPY . .
COPY requirements.txt .

RUN apt-get update 
RUN yes | apt-get install gcc

RUN pip3 install -r requirements.txt --upgrade
