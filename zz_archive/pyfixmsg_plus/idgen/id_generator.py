import datetime
import logging
import string
from typing import Optional


class ClientOrderIdGenerator:
    """
    Abstract base class for client order ID generators.
    """
    def encode(self) -> str:
        raise NotImplementedError("Subclasses must implement encode()")

    def decode(self, to_be_decoded: str) -> int:
        raise NotImplementedError("Subclasses must implement decode()")


class NumericClOrdIdGenerator(ClientOrderIdGenerator):
    """
    Generates unique ClOrdIDs seeded with a passed-in number (e.g. endpoint id).
    """

    def __init__(self, eid: int, length: int = 10, seed: bool = True) -> None:
        self.length = length
        self.uid = 0
        self.seed = seed
        self.max_cl_ord_id = 10**(length - 1)  # Max ClOrdID based on length
        self.endpoint_modulo = 10**(length - 8)  # Endpoint modulo based on length
        self.time_divisor = 86400 // (10**(length - 9))  # Time divisor based on length
        self.init(eid)

    def init(self, eid: int) -> None:
        if self.length < 10:
            raise ValueError("Smallest supported NumericClOrdIdGenerator length is 10")

        self.uid = 0
        self.uid += eid % self.endpoint_modulo

        segment = 1
        if self.seed:
            now = int(datetime.time.time())
            segment = now % 86400
            segment //= self.time_divisor
            segment += 1
            if segment == 5:
                segment += 1

        self.uid += segment * self.endpoint_modulo
        self.uid *= self.max_cl_ord_id
        logging.debug(f"Initialized numeric ClOrdID generator with UID prefix = [{self.uid}] eid [{eid}]")

    def decode(self, to_be_decoded: str) -> int:
        return int(to_be_decoded) - self.uid

    def encode(self, to_be_encoded: int) -> str:
        if to_be_encoded >= self.max_cl_ord_id:
            raise ValueError("Max ClOrdID exceeded")
        return str(self.uid + to_be_encoded)


class YMDClOrdIdGenerator(ClientOrderIdGenerator):
    """
    Generates unique ClOrdIDs with a YMD prefix and a sequence number.
    """
    def __init__(self, eid: int = 0, seed: bool = True) -> None:
        self.eid = eid
        self.seed = seed
        self.counter = 1
        self.ymd_prefix = self.init_ymd_prefix()

    def init_ymd_prefix(self) -> str:
        convert = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
        today = datetime.datetime.now(datetime.UTC)
        year_prefix = convert[today.year % 36]
        month_prefix = convert[today.month]
        day_prefix = convert[today.day]
        return f"{year_prefix}{month_prefix}{day_prefix}-"

    def next_id(self) -> str:
        clordid = f"{self.ymd_prefix}{self.counter}"
        self.counter += 1
        return clordid

    def encode(self) -> str:
        return self.next_id()

    def decode(self, to_be_decoded: str) -> int:
        try:
            return int(to_be_decoded[4:])
        except Exception:
            return -1


class MonthClOrdIdGenerator(NumericClOrdIdGenerator):
    """
    Generates unique ClOrdIDs with a day-of-month prefix.
    """

    def __init__(self, eid: int, seed: bool = True) -> None:
        super().__init__(eid, 13, seed)
        self.day_index = self.init_day_index()

    def init_day_index(self) -> str:
        today = datetime.datetime.now(datetime.UTC).day
        if today < 26:
            return chr(ord('A') + today)
        else:
            return chr(ord('a') + today - 26)

    def decode(self, to_be_decoded: str) -> int:
        return super().decode(to_be_decoded[1:])

    def encode(self, to_be_encoded: int) -> str:
        if to_be_encoded >= self.max_cl_ord_id:
            raise ValueError("Max ClOrdID exceeded")
        cl_ord_id = super().encode(to_be_encoded)
        return f"{self.day_index}{cl_ord_id}"


class NyseBranchSeqGenerator(ClientOrderIdGenerator):
    """
    Generates unique ClOrdIDs with a NYSE branch sequence.
    """

    RESERVED_BRANCH_CODE = staticmethod(lambda Char1, Char2, Char3: (((ord(Char1) - ord('A')) * 26 + (ord(Char2) - ord('A'))) * 26 + (ord(Char3) - ord('A'))) * 10000)

    skipped = [
        RESERVED_BRANCH_CODE('H', 'M', 'Q'),
        RESERVED_BRANCH_CODE('Q', 'Q', 'Q'),
        RESERVED_BRANCH_CODE('R', 'R', 'R'),
        RESERVED_BRANCH_CODE('T', 'T', 'T'),
        RESERVED_BRANCH_CODE('Y', 'Y', 'Y'),
        RESERVED_BRANCH_CODE('Z', 'Y', 'X'),
        RESERVED_BRANCH_CODE('Z', 'Y', 'Y'),
        RESERVED_BRANCH_CODE('Z', 'Y', 'Z'),
        RESERVED_BRANCH_CODE('Z', 'Z', 'Z'),
    ]

    def __init__(self, s: str, sep: str) -> None:
        if not s:
            raise ValueError(f"NyseBranchSeqGenerator: wrong parameter = {s}")
        if len(s) > 7:
            raise ValueError(f"NyseBranchSeqGenerator: wrong parameter = {s}")
        if '-' not in s:
            raise ValueError(f"NyseBranchSeqGenerator: wrong parameter = {s}")

        self.min_: int = self.get_min(s, s.split('-')[0])
        if self.min_ == -1:
            raise ValueError(f"NyseBranchSeqGenerator: wrong parameter = {s}")

        for skipped in self.skipped:
            if self.min_ * 10000 == skipped:
                raise ValueError(f"NyseBranchSeqGenerator: starting branch code is reserved: {s}")

        self.max_: int = self.get_max(s.split('-')[1], s)
        if self.max_ == -1 or self.max_ < self.min_:
            raise ValueError(f"NyseBranchSeqGenerator: wrong parameter = {s}")

        self.min_ *= 10000
        self.max_ = self.max_ * 10000 + 9999
        self.available_ids_: int = self.get_total_num_of_available_ids(self.min_, self.max_)
        self.init_min()
        self.num_of_skips_for_min_: int = self.min_ // 10000

        if self.available_ids_ <= 0:
            raise ValueError(f"NyseBranchSeqGenerator: wrong parameter = {s}")

        logging.info(f"Total available ids for {s} = {self.available_ids_}")
        logging.info(f"Number of skips of min for {s} = {self.num_of_skips_for_min_}")
        logging.info(f"Index skipped by min for {s} = {self.idx_skipped_by_min_}")

        now = datetime.time.localtime()
        self.id_template_: str = f"XXX{sep}NNNN/{datetime.time.strftime('%m%d%Y', now)}"

        logging.info(f"Min = {self.min_}, Max = {self.max_}")

    def get_min(self, p_start: str, p_end: str) -> int:
        if len(p_end) - len(p_start) > 3:
            return -1

        buf = ['A', 'A', 'A']
        i = 0
        for p in p_start:
            if p.isupper():
                buf[i] = p
                i += 1
            else:
                return -1

        return (ord(buf[0]) - ord('A')) * 26 * 26 + (ord(buf[1]) - ord('A')) * 26 + ord(buf[2]) - ord('A')

    def get_max(self, p_start: str, p_end: str) -> int:
        if len(p_end) - len(p_start) > 3:
            return -1

        buf = ['Z', 'Z', 'Z']
        i = 0
        for p in p_start:
            if p.isupper():
                buf[i] = p
                i += 1
            else:
                return -1

        return (ord(buf[0]) - ord('A')) * 26 * 26 + (ord(buf[1]) - ord('A')) * 26 + ord(buf[2]) - ord('A')

    def get_total_num_of_available_ids(self, t_begin: int, t_end: int) -> int:
        num_skips = (t_end - t_begin) // 10000
        for skipped in self.skipped:
            if t_begin > skipped + 9999:
                continue
            elif t_end > skipped:
                num_skips += 9999
            else:
                return t_end - t_begin - num_skips

        return t_end - t_begin - num_skips

    def init_min(self) -> None:
        for i in range(5):
            if self.min_ == self.skipped[i]:
                self.idx_skipped_by_min_ = i + 1
                self.min_ += 10000
                return
            elif self.min_ < self.skipped[i]:
                return
            else:
                self.idx_skipped_by_min_ = i + 1

        self.idx_skipped_by_min_ = 5

        if self.min_ < self.skipped[5]:
            return

        for i in range(3, 0, -1):
            if self.min_ == self.skipped[5 + i]:
                self.min_ += 10000 * i
                return

        self.idx_skipped_by_min_ = 8

    def get_nth_id(self, seqno: int) -> int:
        num_skips = (seqno - 1) // 9999
        ret = seqno + num_skips + self.min_

        if self.idx_skipped_by_min_ == 8:
            return ret

        for i in range(self.idx_skipped_by_min_, 5):
            if ret < self.skipped[i]:
                return ret
            else:
                ret += 10000

        if ret < self.skipped[5]:
            return ret

        if ret < self.skipped[8]:
            return ret + 30000

        return -1

    def decode(self, to_be_decoded: str) -> int:
        if not to_be_decoded or len(to_be_decoded) != 17:
            return -1

        num = 0
        alpha = 0

        for p in to_be_decoded[4:8]:
            if not p.isdigit():
                return -1
            num = num * 10 + int(p)

        for p in to_be_decoded[:3]:
            if not p.isupper():
                return -1
            alpha = alpha * 26 + (ord(p) - ord('A'))

        return self.get_total_num_of_available_ids(self.min_, alpha * 10000 + num)

    def encode(self, to_be_encoded: int) -> str:
        if to_be_encoded > self.available_ids_:
            return ""

        to_be_encoded = self.get_nth_id(to_be_encoded)
        encoded = list(self.id_template_)

        num = to_be_encoded % 10000
        alpha = to_be_encoded // 10000

        for p in range(7, 3, -1):
            rem = num % 10
            encoded[p] = chr(ord('0') + rem)
            num //= 10

        for p in range(2, -1, -1):
            rem = alpha % 26
            encoded[p] = chr(ord('A') + rem)
            alpha //= 26

        return "".join(encoded)


class OSESeqGenerator(ClientOrderIdGenerator):
    """
    Generates unique ClOrdIDs with a fixed prefix.
    """

    def __init__(self, prefix: str) -> None:
        if not prefix or len(prefix) > 10:
            raise ValueError("Prefix length should be less than or equal to 10.")
        self.id_prefix = prefix.ljust(10, '0')

    def encode(self, to_be_encoded: int) -> str:
        if not isinstance(to_be_encoded, int):
            raise ValueError("Input should be an integer.")
        return f"{self.id_prefix}{to_be_encoded:010d}"

    def decode(self, to_be_decoded: str) -> int:
        if not to_be_decoded or len(to_be_decoded) != 20:
            return -1
        try:
            return int(to_be_decoded[10:])
        except ValueError:
            return -1


class CHIXBranchSeqGenerator(NyseBranchSeqGenerator):
    """
    Generates unique ClOrdIDs with a CHIX branch sequence.
    """

    def __init__(self, s: str) -> None:
        super().__init__(s, '_')
