#!/usr/bin/env Rint
library(grDevices)
library(utils)
X11(width=12, height=10)

library(ggplot2)
args <- commandArgs(trailingOnly = TRUE)
args <- if (length(args) == 0) Sys.getenv("ARGS") else args
args <- if (args[1] == "") "plot.dat" else args
d <- data.frame(read.table(
			   text=grep('^mx[123]', readLines(file(args[1])), value=T),
			   col.names=c("lock", "cores", "readers", "iterations", "wprob", "wwork", "rwork", "refresh", "time", "X"),
			   colClasses=c(rep(NA, 9), rep("NULL"))
			   ))
d$ops = 1/(d$time/d$iterations/d$readers)
d$lock = sapply(d$lock, function(x) { if (x == "mx1") "sync.RWMutex" else "DRWMutex (CPUID)" })
da <- aggregate(d$ops, by = list(
				 lock=d$lock,
				 cores=d$cores,
				 refresh=d$refresh,
				 rwork=d$rwork,
				 wprob=d$wprob,
				 readers=d$readers), FUN = summary)
p <- ggplot(data=da, aes(x = cores, y = x[,c("Mean")], ymin = x[,c("1st Qu.")], ymax = x[,c("3rd Qu.")], color = lock))
p <- p + geom_line()
p <- p + geom_errorbar()
#p <- p + facet_wrap(~ readers)
#p <- p + facet_grid(refresh ~ rwork, labeller = function(var, val) {paste(var, " = ", val)})
#p <- p + geom_smooth()
p <- p + xlab("CPU cores")
p <- p + ylab("Mean ops per second per reader")

p
ggsave("perf.png", plot = p, width = 8, height = 6)
