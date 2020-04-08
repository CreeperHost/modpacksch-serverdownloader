package net.creeperhost.creeperlauncher.util;

import java.io.*;
import java.nio.file.*;
import java.util.Comparator;

public class FileUtils
{
    public static boolean deleteDirectory(File file)
    {
        try
        {
            Files.walk(file.toPath())
                    .sorted(Comparator.reverseOrder())
                    .map(Path::toFile)
                    .forEach(File::delete);

        } catch (IOException ignored) {}
        return file.delete();
    }
}
